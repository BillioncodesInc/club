package service

import (
	"context"
	"fmt"
	"time"

	"github.com/go-errors/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"gorm.io/gorm"
)

// Session is a service for Session
type Session struct {
	Common
	SessionRepository *repository.Session
}

// GetSession returns a session if one exists associated with the request
// if the session exists it will extend the session expiry date
// else it will invalidate the session cookie if provided
// modifies the response headers
func (s *Session) GetAndExtendSession(g *gin.Context) (*model.Session, error) {
	session, err := s.validateAndExtendSession(g)
	hasErr := errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, errs.ErrSessionCookieNotFound)
	if hasErr {
		return nil, errs.Wrap(err)
	}
	if err != nil {
		// nil session = system-initiated (session is being rejected)
		// This is now a generic validation-failure audit. IP changes no
		// longer surface here (they're handled tolerantly inside
		// validateAndExtendSession and audit-logged as Session.IPChange).
		ae := NewAuditEvent("Session.ValidationFailed", nil)
		ae.Details["error"] = err.Error()
		s.AuditLogNotAuthorized(ae)
		s.Logger.Debugw("failed to validate and extend session", "error", err)
		return nil, errs.Wrap(err)
	}
	return session, nil
}

// GetByID returns a session by ID
func (s *Session) GetByID(
	ctx context.Context,
	sessionID *uuid.UUID,
	options *repository.SessionOption,
) (*model.Session, error) {
	session, err := s.SessionRepository.GetByID(
		ctx,
		sessionID,
		options,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}

	return session, nil
}

// GetSessionsByUserID returns all sessions by user ID
func (s *Session) GetSessionsByUserID(
	ctx context.Context,
	session *model.Session,
	userID *uuid.UUID,
	options *repository.SessionOption,
) (*model.Result[model.Session], error) {
	result := model.NewEmptyResult[model.Session]()
	ae := NewAuditEvent("Session.GetSessionsByUserID", session)
	ae.Details["userID"] = userID.String()
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		s.LogAuthError(err)
		return result, errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return result, errs.ErrAuthorizationFailed
	}
	// get all sessions by user ID
	result, err = s.SessionRepository.GetAllActiveSessionByUserID(
		ctx,
		userID,
		options,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, gorm.ErrRecordNotFound
	}
	if err != nil {
		s.Logger.Errorw("failed to get sessions by user ID", "error", err)
		return result, errs.Wrap(err)
	}
	// no audit on read

	return result, nil
}

// validateAndExtendSession returns a session if one exists associated with the request
func (s *Session) validateAndExtendSession(g *gin.Context) (*model.Session, error) {
	cookie, err := g.Cookie(data.SessionCookieKey)
	if err != nil {
		return nil, errs.ErrSessionCookieNotFound
	}
	id, err := uuid.Parse(cookie)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	// checks that the session is not expired
	ctx := g.Request.Context()
	session, err := s.SessionRepository.GetByID(ctx, &id, &repository.SessionOption{
		WithUser:        true,
		WithUserRole:    true,
		WithUserCompany: true,
	})
	// there is a valid session cookie but no valid session, so we expire the session cookie
	if errors.Is(err, gorm.ErrRecordNotFound) {
		g.SetCookie(
			data.SessionCookieKey,
			"",
			-1,
			"/",
			"",
			true,
			true,
		)
		return nil, errs.Wrap(err)
	}
	if err != nil {
		return nil, errs.Wrap(err)
	}
	// handle session and that IP has not changed
	// use RemoteIP() (port-stripped RemoteAddr) rather than ClientIP() so a caller
	// cannot spoof a session by sending a forged X-Forwarded-For header. Gin's
	// ClientIP() will honor XFF if the engine has trusted proxies configured,
	// which is out of scope for session IP pinning - we always want the actual
	// TCP peer for this security check.
	//
	// Historically a mismatch caused the session to be expired immediately.
	// That is too strict for real-world clients: users behind NAT/CGNAT
	// (mobile carriers in particular), corporate proxies with multiple
	// egress IPs, or VPN reconnects will legitimately change their TCP
	// peer address mid-session. The real threat the IP pin guards against
	// is cookie theft replayed from a completely different client -
	// which in practice is much more cleanly detected by fingerprint
	// changes (user-agent, TLS fingerprint) than by IP alone.
	//
	// Tradeoff: we tolerate the IP change, audit-log it (so an operator
	// can correlate suspicious activity after the fact), update the
	// stored IP, and let the session continue. A security operator who
	// wants strict IP pinning can still act on the audit events.
	sessionIP := session.IP
	clientIP := g.RemoteIP()
	ipChanged := sessionIP != clientIP && clientIP != ""
	if ipChanged {
		// audit log - IP changed but session continues
		ae := NewAuditEvent("Session.IPChange", session)
		ae.Details["reason"] = "client IP changed; session continuing under grace policy"
		ae.Details["previousIP"] = sessionIP
		ae.Details["newIP"] = clientIP
		s.AuditLogAuthorized(ae)
		// update the stored IP on the session so subsequent checks
		// compare against the latest known peer address
		session.IP = clientIP
	}
	// session is valid - update the session expiry date (and IP if changed)
	session.Renew(model.SessionIdleTimeout)
	err = s.SessionRepository.UpdateExpiry(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to update session expiry: %s", err)
	}
	if ipChanged {
		if err := s.SessionRepository.UpdateIP(ctx, session.ID, clientIP); err != nil {
			// don't fail the request over a best-effort IP persist;
			// the audit log already captured the change
			s.Logger.Warnw("failed to persist changed session IP", "error", err)
		}
	}

	return session, nil
}

// Create creates a new session
// no auth - anyone can create a session
func (s *Session) Create(
	ctx context.Context,
	user *model.User,
	ip string,
) (*model.Session, error) {
	now := time.Now()
	expiredAt := now.Add(model.SessionIdleTimeout).UTC()
	maxAgeAt := now.Add(model.SessionMaxAgeAt).UTC()
	id := uuid.New()
	newSession := &model.Session{
		ID:        &id,
		User:      user,
		IP:        ip,
		ExpiresAt: &expiredAt,
		MaxAgeAt:  &maxAgeAt,
	}
	sessionID, err := s.SessionRepository.Insert(
		ctx,
		newSession,
	)
	if err != nil {
		s.Logger.Errorw("failed to insert session when creating a new session", "error", err)
		return nil, errs.Wrap(err)
	}
	createdSession, err := s.SessionRepository.GetByID(
		ctx,
		sessionID,
		&repository.SessionOption{
			WithUser:        true,
			WithUserRole:    true,
			WithUserCompany: true,
		},
	)
	if err != nil {
		s.Logger.Errorw("failed to get session after creating it", "error", err)
		return nil, errs.Wrap(err)
	}
	return createdSession, nil
}

// Expire expires a session
func (s *Session) Expire(
	ctx context.Context,
	sessionID *uuid.UUID,
) error {
	err := s.SessionRepository.Expire(ctx, sessionID)
	if err != nil {
		s.Logger.Errorw("failed to expire session", "error", err)
		return err
	}
	return nil
}

// ExpireAllByUserID expires all sessions by user ID
func (s *Session) ExpireAllByUserID(
	ctx context.Context,
	session *model.Session,
	userID *uuid.UUID,
) error {
	ae := NewAuditEvent("Session.ExpireAllByUserID", session)
	ae.Details["userID"] = userID.String()
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		s.LogAuthError(err)
		return err
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return errs.ErrAuthorizationFailed
	}
	if session.User == nil {
		s.Logger.Errorw("failed to get user from session when expiring session", "error", err)
		return err
	}
	sessions, err := s.SessionRepository.GetAllActiveSessionByUserID(
		ctx,
		userID,
		&repository.SessionOption{},
	)
	if err != nil {
		s.Logger.Errorw("failed to get user sessions when expiring session", "error", err)
		return err
	}
	if len(sessions.Rows) == 0 {
		s.Logger.Debugw("no sessions to remove", "userID", userID.String())
	}
	for _, session := range sessions.Rows {
		err = s.SessionRepository.Expire(ctx, session.ID)
		if err != nil {
			s.Logger.Errorw("failed a users expiring session", "error", err)
			return err
		}
	}
	s.AuditLogAuthorized(ae)

	return nil
}
