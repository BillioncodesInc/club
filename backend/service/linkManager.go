package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"go.uber.org/zap"
)

// ClickEntry records a single click event on a short link
type ClickEntry struct {
	Timestamp time.Time `json:"timestamp"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"userAgent"`
	Referer   string    `json:"referer"`
	Country   string    `json:"country"`
}

// ShortLink represents a shortened URL with tracking
type ShortLink struct {
	Code        string       `json:"code"`
	OriginalURL string       `json:"originalUrl"`
	ShortURL    string       `json:"shortUrl"`
	Created     time.Time    `json:"created"`
	Expires     *time.Time   `json:"expires,omitempty"`
	Clicks      int          `json:"clicks"`
	ClickLog    []ClickEntry `json:"clickLog,omitempty"`
	CampaignID  string       `json:"campaignId,omitempty"`
	ProxyID     string       `json:"proxyId,omitempty"`
	DomainID    string       `json:"domainId,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
}

// LinkAnalytics provides aggregated analytics for a short link
type LinkAnalytics struct {
	Code         string         `json:"code"`
	OriginalURL  string         `json:"originalUrl"`
	TotalClicks  int            `json:"totalClicks"`
	UniqueClicks int            `json:"uniqueClicks"`
	ByCountry    map[string]int `json:"byCountry"`
	ByHour       map[int]int    `json:"byHour"`
	LastClick    *time.Time     `json:"lastClick,omitempty"`
	Created      time.Time      `json:"created"`
}

// ShortenRequest is the request to create a shortened URL
type ShortenRequest struct {
	URL        string   `json:"url"`
	CustomCode string   `json:"customCode,omitempty"`
	CodeLength int      `json:"codeLength,omitempty"`
	Domain     string   `json:"domain,omitempty"`
	ExpiresIn  int      `json:"expiresIn,omitempty"` // seconds
	CampaignID string   `json:"campaignId,omitempty"`
	ProxyID    string   `json:"proxyId,omitempty"`
	DomainID   string   `json:"domainId,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

// ProxyLinkInfo holds information about a link derived from a proxy configuration
type ProxyLinkInfo struct {
	ProxyID     string `json:"proxyId"`
	ProxyName   string `json:"proxyName"`
	DomainID    string `json:"domainId,omitempty"`
	DomainName  string `json:"domainName,omitempty"`
	StartURL    string `json:"startUrl"`
	PhishingURL string `json:"phishingUrl,omitempty"`
}

// LinkManager provides URL shortening, link tracking, and rotation.
// It integrates with Phishing Club's proxy/domain system so that links
// created through the proxy YAML editor are also visible and manageable here.
type LinkManager struct {
	Common
	mu               sync.RWMutex
	links            map[string]*ShortLink
	ProxyRepository  *repository.Proxy
	DomainRepository *repository.Domain
}

// NewLinkManager creates a new LinkManager instance
func NewLinkManager(
	logger *zap.SugaredLogger,
	proxyRepository *repository.Proxy,
	domainRepository *repository.Domain,
) *LinkManager {
	return &LinkManager{
		Common:           Common{Logger: logger},
		links:            make(map[string]*ShortLink),
		ProxyRepository:  proxyRepository,
		DomainRepository: domainRepository,
	}
}

// GetProxyLinks returns all links derived from proxy configurations and their
// associated domains. This bridges the proxy YAML editor with the link manager,
// so users can see and manage links created through the proxy tab.
func (lm *LinkManager) GetProxyLinks(
	ctx context.Context,
	session *model.Session,
) ([]ProxyLinkInfo, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	// Get all proxies (overview subset)
	proxyResult, err := lm.ProxyRepository.GetAllSubset(
		ctx,
		nil, // no company filter
		&repository.ProxyOption{},
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// Get all domains (overview subset) to map proxy -> domain
	domainResult, err := lm.DomainRepository.GetAllSubset(
		ctx,
		nil, // no company filter
		&repository.DomainOption{},
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// Build proxy ID -> domain mapping
	proxyDomainMap := make(map[string]*model.DomainOverview)
	for _, domain := range domainResult.Rows {
		if domain.ProxyID != nil {
			proxyDomainMap[domain.ProxyID.String()] = domain
		}
	}

	var proxyLinks []ProxyLinkInfo
	for _, proxy := range proxyResult.Rows {
		info := ProxyLinkInfo{
			ProxyID:   proxy.ID.String(),
			ProxyName: proxy.Name,
			StartURL:  proxy.StartURL,
		}

		// Check if a domain is linked to this proxy
		if domain, ok := proxyDomainMap[proxy.ID.String()]; ok {
			info.DomainID = domain.ID.String()
			info.DomainName = domain.Name
			info.PhishingURL = "https://" + domain.Name
		}

		proxyLinks = append(proxyLinks, info)
	}

	return proxyLinks, nil
}

// ShortenFromProxy creates a short link that wraps a proxy's phishing URL.
// This allows the link manager to create trackable short URLs for links
// that were originally created through the proxy YAML editor.
func (lm *LinkManager) ShortenFromProxy(
	ctx context.Context,
	session *model.Session,
	proxyID string,
	customCode string,
	codeLength int,
	expiresIn int,
	tags []string,
) (*ShortLink, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	// Get the proxy
	proxyUUID, err := uuid.Parse(proxyID)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy ID: %s", proxyID)
	}
	proxy, err := lm.ProxyRepository.GetByID(ctx, &proxyUUID, &repository.ProxyOption{})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	startURL := proxy.StartURL.MustGet().String()

	// Find the domain linked to this proxy
	domainResult, err := lm.DomainRepository.GetAllSubset(ctx, nil, &repository.DomainOption{})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	var phishingDomain string
	var domainID string
	for _, domain := range domainResult.Rows {
		if domain.ProxyID != nil && domain.ProxyID.String() == proxyID {
			phishingDomain = domain.Name
			domainID = domain.ID.String()
			break
		}
	}

	// Build the original URL from the proxy's phishing domain + start URL path
	originalURL := startURL
	if phishingDomain != "" {
		parsedStart, err := url.Parse(startURL)
		if err == nil {
			originalURL = "https://" + phishingDomain + parsedStart.Path
		}
	}

	// Use the phishing domain for the short URL base
	shortDomain := "https://" + phishingDomain
	if phishingDomain == "" {
		shortDomain = "https://localhost"
	}

	req := &ShortenRequest{
		URL:        originalURL,
		CustomCode: customCode,
		CodeLength: codeLength,
		Domain:     shortDomain,
		ExpiresIn:  expiresIn,
		CampaignID: "",
		ProxyID:    proxyID,
		DomainID:   domainID,
		Tags:       tags,
	}

	return lm.Shorten(req)
}

// Shorten creates a shortened URL
func (lm *LinkManager) Shorten(req *ShortenRequest) (*ShortLink, error) {
	if req.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}

	codeLength := req.CodeLength
	if codeLength <= 0 {
		codeLength = 6
	}

	code := req.CustomCode
	if code == "" {
		code = generateShortCode(codeLength)
	}

	domain := req.Domain
	// if domain is not set but domainId is provided, resolve the domain name from DB
	if domain == "" && req.DomainID != "" && lm.DomainRepository != nil {
		domainUUID, err := uuid.Parse(req.DomainID)
		if err == nil {
			ctx := context.Background()
			dbDomain, err := lm.DomainRepository.GetByID(ctx, &domainUUID, &repository.DomainOption{})
			if err == nil && dbDomain != nil {
				if name, err := dbDomain.Name.Get(); err == nil {
					domain = "https://" + name.String()
				}
			}
		}
	}
	if domain == "" {
		domain = "http://localhost"
	}
	domain = strings.TrimRight(domain, "/")

	var expires *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
		expires = &t
	}

	link := &ShortLink{
		Code:        code,
		OriginalURL: req.URL,
		ShortURL:    fmt.Sprintf("%s/%s", domain, code),
		Created:     time.Now(),
		Expires:     expires,
		Clicks:      0,
		ClickLog:    []ClickEntry{},
		CampaignID:  req.CampaignID,
		ProxyID:     req.ProxyID,
		DomainID:    req.DomainID,
		Tags:        req.Tags,
	}

	lm.mu.Lock()
	lm.links[code] = link
	lm.mu.Unlock()

	lm.Logger.Infow("created short link",
		"code", code,
		"originalUrl", req.URL,
		"shortUrl", link.ShortURL,
		"proxyId", req.ProxyID,
		"domainId", req.DomainID,
	)

	return link, nil
}

// Expand resolves a short code to the original URL
func (lm *LinkManager) Expand(code string) (string, error) {
	lm.mu.RLock()
	link, exists := lm.links[code]
	lm.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("link not found: %s", code)
	}

	if link.Expires != nil && link.Expires.Before(time.Now()) {
		lm.mu.Lock()
		delete(lm.links, code)
		lm.mu.Unlock()
		return "", fmt.Errorf("link expired: %s", code)
	}

	return link.OriginalURL, nil
}

// TrackClick records a click event on a short link
func (lm *LinkManager) TrackClick(code string, ip, userAgent, referer, country string) (*ShortLink, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	link, exists := lm.links[code]
	if !exists {
		return nil, fmt.Errorf("link not found: %s", code)
	}

	link.Clicks++
	link.ClickLog = append(link.ClickLog, ClickEntry{
		Timestamp: time.Now(),
		IP:        ip,
		UserAgent: userAgent,
		Referer:   referer,
		Country:   country,
	})

	return link, nil
}

// GetAnalytics returns aggregated analytics for a short link
func (lm *LinkManager) GetAnalytics(code string) (*LinkAnalytics, error) {
	lm.mu.RLock()
	link, exists := lm.links[code]
	lm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("link not found: %s", code)
	}

	byCountry := make(map[string]int)
	byHour := make(map[int]int)
	uniqueIPs := make(map[string]bool)

	for _, click := range link.ClickLog {
		country := click.Country
		if country == "" {
			country = "unknown"
		}
		byCountry[country]++
		byHour[click.Timestamp.Hour()]++
		uniqueIPs[click.IP] = true
	}

	var lastClick *time.Time
	if len(link.ClickLog) > 0 {
		t := link.ClickLog[len(link.ClickLog)-1].Timestamp
		lastClick = &t
	}

	return &LinkAnalytics{
		Code:         link.Code,
		OriginalURL:  link.OriginalURL,
		TotalClicks:  link.Clicks,
		UniqueClicks: len(uniqueIPs),
		ByCountry:    byCountry,
		ByHour:       byHour,
		LastClick:    lastClick,
		Created:      link.Created,
	}, nil
}

// RotateLinks changes the destination URL for multiple short codes
func (lm *LinkManager) RotateLinks(codes []string, newURL string) []map[string]interface{} {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	var results []map[string]interface{}
	for _, code := range codes {
		link, exists := lm.links[code]
		if exists {
			link.OriginalURL = newURL
			results = append(results, map[string]interface{}{
				"code":    code,
				"newUrl":  newURL,
				"updated": true,
			})
		} else {
			results = append(results, map[string]interface{}{
				"code":    code,
				"updated": false,
			})
		}
	}

	lm.Logger.Infow("rotated links",
		"count", len(codes),
		"newUrl", newURL,
	)

	return results
}

// GetAllLinks returns all short links, optionally filtered by campaign or proxy
func (lm *LinkManager) GetAllLinks(campaignID string, proxyID string) []*ShortLink {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var links []*ShortLink
	for _, link := range lm.links {
		if campaignID != "" && link.CampaignID != campaignID {
			continue
		}
		if proxyID != "" && link.ProxyID != proxyID {
			continue
		}
		links = append(links, link)
	}
	return links
}

// DeleteLink removes a short link
func (lm *LinkManager) DeleteLink(code string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if _, exists := lm.links[code]; !exists {
		return fmt.Errorf("link not found: %s", code)
	}

	delete(lm.links, code)
	return nil
}

func generateShortCode(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:length]
}
