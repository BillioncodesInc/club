package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	"github.com/gin-gonic/gin"
	"github.com/go-errors/errors"

	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/random"
	"github.com/phishingclub/phishingclub/sso"
	"github.com/phishingclub/phishingclub/vo"
)

// ssoStateTTL is the lifetime of an SSO oauth state token
const ssoStateTTL = 10 * time.Minute

type SSO struct {
	Common
	OptionsService *Option
	UserService    *User
	SessionService *Session
	MSALClient     *confidential.Client

	// ssoState stores oauth state tokens for csrf protection on the SSO callback
	// key = state token, value = expiry time
	ssoStateMu sync.Mutex
	ssoState   map[string]time.Time
}

// storeSSOState stores an oauth state token with a TTL
func (s *SSO) storeSSOState(stateToken string) {
	s.ssoStateMu.Lock()
	defer s.ssoStateMu.Unlock()
	if s.ssoState == nil {
		s.ssoState = map[string]time.Time{}
	}
	// opportunistic cleanup of expired entries
	now := time.Now()
	for k, exp := range s.ssoState {
		if now.After(exp) {
			delete(s.ssoState, k)
		}
	}
	s.ssoState[stateToken] = now.Add(ssoStateTTL)
}

// consumeSSOState validates and removes an oauth state token
// returns true if the token was valid and unexpired
func (s *SSO) consumeSSOState(stateToken string) bool {
	if stateToken == "" {
		return false
	}
	s.ssoStateMu.Lock()
	defer s.ssoStateMu.Unlock()
	if s.ssoState == nil {
		return false
	}
	exp, ok := s.ssoState[stateToken]
	if !ok {
		return false
	}
	delete(s.ssoState, stateToken)
	if time.Now().After(exp) {
		return false
	}
	return true
}

type MsGraphUserInfo struct {
	DisplayName       string `json:"displayName"`       // Full name
	Email             string `json:"mail"`              // Primary email
	UserPrincipalName string `json:"userPrincipalName"` // Often email or login
	GivenName         string `json:"givenName"`         // First name
	Surname           string `json:"surname"`           // Last name
	ID                string `json:"id"`                // Unique Azure AD ID
}

// Get is the auth protected method for getting SSO details
func (s *SSO) Get(
	ctx context.Context,
	session *model.Session,
) (*model.SSOOption, error) {
	ae := NewAuditEvent("SSO.Get", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
	return s.GetSSOOptionWithoutAuth(ctx)
}

// Upsert upserts SSO config it also replaces the in memory SSO configuration
func (s *SSO) Upsert(
	ctx context.Context,
	session *model.Session,
	ssoOpt *model.SSOOption,
) error {
	ae := NewAuditEvent("SSO.Upsert", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		s.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return errs.ErrAuthorizationFailed
	}
	ssoOpt.Enabled = len(ssoOpt.ClientID.String()) > 0 &&
		len(ssoOpt.TenantID.String()) > 0 &&
		len(ssoOpt.ClientSecret.String()) > 0

	// if the config is incomplete, we clear it
	if !ssoOpt.Enabled {
		ssoOpt.ClientID = *vo.NewEmptyOptionalString64()
		ssoOpt.TenantID = *vo.NewEmptyOptionalString64()
		ssoOpt.ClientSecret = *vo.NewEmptyOptionalString1024()
		ssoOpt.RedirectURL = *vo.NewEmptyOptionalString1024()
	}
	opt, err := ssoOpt.ToOption()
	if err != nil {
		return errs.Wrap(err)
	}
	err = s.OptionsService.SetOptionByKey(ctx, session, opt)
	if err != nil {
		s.Logger.Errorw("failed to upsert sso option", "error", err)
		return errs.Wrap(err)
	}
	s.AuditLogAuthorized(ae)
	// replace the in memory msal client
	if ssoOpt.Enabled {
		s.MSALClient, err = sso.NewEntreIDClient(ssoOpt)
		if err != nil {
			return errs.Wrap(err)
		}
	} else {
		s.MSALClient = nil
	}

	return nil
}

func (s *SSO) GetSSOOptionWithoutAuth(ctx context.Context) (*model.SSOOption, error) {
	opt, err := s.OptionsService.GetOptionWithoutAuth(ctx, data.OptionKeyAdminSSOLogin)
	if err != nil {
		s.Logger.Errorw("failed to get sso option",
			"key", data.OptionKeyAdminSSOLogin,
			"error", err)
		return nil, errs.Wrap(err)
	}
	ssoOpt, err := model.NewSSOOptionFromJSON([]byte(opt.Value.String()))
	if err != nil {
		s.Logger.Errorw("failed to unmarshall sso option", "error", err)
		return nil, errs.Wrap(err)
	}
	return ssoOpt, nil
}

func (s *SSO) EntreIDLogin(ctx context.Context) (string, error) {
	// check if sso is enabled
	ssoOpt, err := s.GetSSOOptionWithoutAuth(ctx)
	if err != nil {
		return "", err
	}
	if !ssoOpt.Enabled {
		s.Logger.Debugf("SSO login URL visited but it is disabed")
		return "", errs.Wrap(errs.ErrSSODisabled)
	}
	// the MSALCLient is set on application start up
	// and when a upsert is done, replacing the old client with new details
	if s.MSALClient == nil {
		return "", errs.Wrap(errors.New("no MSAL client"))
	}
	authURL, err := s.MSALClient.AuthCodeURL(
		ctx,
		ssoOpt.ClientID.String(),
		ssoOpt.RedirectURL.String(),
		[]string{"https://graph.microsoft.com/User.Read"},
	)
	if err != nil {
		return "", errs.Wrap(err)
	}
	// generate cryptographic state token for CSRF protection on the callback
	// msal-go does not support setting the state param via its AuthCodeURL options
	// so we append it manually to the generated URL
	stateToken, err := random.GenerateRandomURLBase64Encoded(32)
	if err != nil {
		s.Logger.Errorw("failed to generate SSO state token", "error", err)
		return "", errs.Wrap(err)
	}
	s.storeSSOState(stateToken)
	if strings.Contains(authURL, "?") {
		authURL += "&state=" + url.QueryEscape(stateToken)
	} else {
		authURL += "?state=" + url.QueryEscape(stateToken)
	}
	return authURL, nil
}

// EntreIDCallBack checks if the callback is OK then requests user details from the graph API
func (s *SSO) HandlEntraIDCallback(
	g *gin.Context,
	code string,
	state string,
) (*model.Session, error) {
	// validate oauth state to prevent CSRF - reject if missing/expired, consume on use
	if !s.consumeSSOState(state) {
		s.Logger.Warnw("SSO callback rejected: invalid or expired state token")
		return nil, errs.Wrap(errors.New("invalid or expired state token"))
	}
	ssoOpt, err := s.GetSSOOptionWithoutAuth(g.Request.Context())
	if err != nil {
		return nil, err
	}
	if !ssoOpt.Enabled {
		return nil, errs.Wrap(errs.ErrSSODisabled)
	}
	if s.MSALClient == nil {
		return nil, errors.New("no msal client in memory")
	}
	result, err := s.MSALClient.AcquireTokenByAuthCode(
		g.Request.Context(),
		code,
		ssoOpt.RedirectURL.String(),
		[]string{"User.Read"},
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	userInfo, err := s.getMsGraphMe(g.Request.Context(), result.AccessToken)
	if err != nil {
		s.Logger.Debugw("failed to get /me graph info", "error", err)
		return nil, err
	}
	// validate required fields
	if userInfo.Email == "" && userInfo.UserPrincipalName == "" {
		err := errors.New("no email provided from SSO")
		s.Logger.Debugw("no email or userPrincipalName from SSO", "error", err)
		return nil, errs.Wrap(err)
	}
	// determine email (prefer mail over UPN)
	email := userInfo.Email
	if email == "" {
		email = userInfo.UserPrincipalName
	}
	// determine name
	name := userInfo.DisplayName
	if name == "" {
		name = strings.TrimSpace(fmt.Sprintf("%s %s", userInfo.GivenName, userInfo.Surname))
	}
	if name == "" {
		// Fallback to email prefix if no name available
		name = strings.Split(email, "@")[0]
	}
	userID, err := s.UserService.CreateFromSSO(g.Request.Context(), name, email, userInfo.ID)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	if userID == nil {
		return nil, errs.Wrap(errors.New("user ID is unexpectedly nil"))
	}
	// get the user and create a session
	user, err := s.UserService.GetByIDWithoutAuth(g.Request.Context(), userID)
	if err != nil {
		s.Logger.Debugf("failed to get SSO user", "error", err)
		return nil, errs.Wrap(err)
	}
	session, err := s.SessionService.Create(g.Request.Context(), user, g.ClientIP())
	if err != nil {
		s.Logger.Debugf("failed to create session from SSO", "error", err)
		return nil, errs.Wrap(err)
	}
	return session, nil
}

func (s *SSO) getMsGraphMe(ctx context.Context, accessToken string) (*MsGraphUserInfo, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := client.Do(req)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errs.Wrap(fmt.Errorf("graph API returned status %d", resp.StatusCode))
	}

	// Read and log raw response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	s.Logger.Debugw("Raw Microsoft Graph response", "body", string(body))

	var userInfo MsGraphUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, errs.Wrap(err)
	}

	s.Logger.Debugw("Parsed user info",
		"id", userInfo.ID,
		"email", userInfo.Email,
		"displayName", userInfo.DisplayName,
		"userPrincipalName", userInfo.UserPrincipalName,
	)

	return &userInfo, nil
}
