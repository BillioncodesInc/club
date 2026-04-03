package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/database"
	"github.com/phishingclub/phishingclub/vo"
	"gorm.io/gorm"
)

var proxyCaptureAllowedColumns = assignTableToColumns(database.PROXY_CAPTURE_TABLE, []string{
	"created_at",
	"updated_at",
	"ip_address",
	"username",
	"phish_domain",
	"target_domain",
})

// ProxyCaptureOption is for eager loading and query options
type ProxyCaptureOption struct {
	*vo.QueryArgs
}

// ProxyCapture is a proxy capture repository
type ProxyCapture struct {
	DB *gorm.DB
}

// Insert inserts a proxy capture record
func (m *ProxyCapture) Insert(
	ctx context.Context,
	capture *database.ProxyCapture,
) (*uuid.UUID, error) {
	id := uuid.New()
	capture.ID = &id
	res := m.DB.Create(capture)
	if res.Error != nil {
		return nil, res.Error
	}
	return &id, nil
}

// UpsertBySessionID finds an existing capture for the given session ID and merges
// new data into it, or inserts a new record if none exists. This ensures that
// username, password, and cookies captured by different rules (or at different times)
// are merged into a single record per session.
func (m *ProxyCapture) UpsertBySessionID(
	ctx context.Context,
	capture *database.ProxyCapture,
) (*uuid.UUID, error) {
	if capture.SessionID == "" {
		return m.Insert(ctx, capture)
	}

	var existing database.ProxyCapture
	res := m.DB.Where("session_id = ?", capture.SessionID).First(&existing)
	if res.Error != nil {
		// no existing record, insert new
		return m.Insert(ctx, capture)
	}

	// merge: only overwrite fields that are non-empty in the new capture
	if capture.Username != "" {
		existing.Username = capture.Username
	}
	if capture.Password != "" {
		existing.Password = capture.Password
	}
	// Merge cookies: accumulate all cookies into a JSON array instead of overwriting.
	// Each cookie capture adds to the array, building a complete session cookie jar.
	if capture.Cookies != "" {
		existing.Cookies = mergeCookiesJSON(existing.Cookies, capture.Cookies)
	}
	if capture.CapturedData != "" {
		existing.CapturedData = capture.CapturedData
	}
	if capture.UserAgent != "" {
		existing.UserAgent = capture.UserAgent
	}
	if capture.ProxyID != nil {
		existing.ProxyID = capture.ProxyID
	}
	existing.UpdatedAt = capture.UpdatedAt

	res = m.DB.Save(&existing)
	if res.Error != nil {
		return nil, res.Error
	}
	return existing.ID, nil
}

// mergeCookiesJSON merges new cookie JSON data into an existing cookie JSON array.
// Both existing and incoming can be:
//   - A JSON object (single cookie): {"name":"X","value":"Y",...}
//   - A JSON array of cookie objects: [{"name":"X",...},{"name":"Y",...}]
//
// The result is always a JSON array. Cookies with the same "name" and "domain"
// are updated (latest value wins), while new cookies are appended.
func mergeCookiesJSON(existingJSON, newJSON string) string {
	existingCookies := parseCookiesJSON(existingJSON)
	newCookies := parseCookiesJSON(newJSON)

	// Build a map keyed by "name:domain" for deduplication
	cookieMap := make(map[string]map[string]interface{})
	// Maintain insertion order
	var orderedKeys []string

	for _, c := range existingCookies {
		key := cookieKey(c)
		if _, exists := cookieMap[key]; !exists {
			orderedKeys = append(orderedKeys, key)
		}
		cookieMap[key] = c
	}

	for _, c := range newCookies {
		key := cookieKey(c)
		if _, exists := cookieMap[key]; !exists {
			orderedKeys = append(orderedKeys, key)
		}
		cookieMap[key] = c // overwrite with latest value
	}

	// Build result array in order
	result := make([]map[string]interface{}, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		result = append(result, cookieMap[key])
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		// fallback: return the new JSON if marshaling fails
		return newJSON
	}
	return string(jsonBytes)
}

// parseCookiesJSON parses a JSON string that can be either a single cookie object
// or an array of cookie objects, and returns a slice of cookie maps.
func parseCookiesJSON(jsonStr string) []map[string]interface{} {
	if jsonStr == "" {
		return nil
	}

	// Try parsing as array first
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &arr); err == nil {
		return arr
	}

	// Try parsing as single object
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &obj); err == nil {
		return []map[string]interface{}{obj}
	}

	return nil
}

// cookieKey generates a unique key for a cookie based on name and domain
func cookieKey(cookie map[string]interface{}) string {
	name, _ := cookie["name"].(string)
	domain, _ := cookie["domain"].(string)
	if name == "" {
		// fallback: use the full JSON as key
		b, _ := json.Marshal(cookie)
		return string(b)
	}
	return name + ":" + domain
}

// GetAll gets all proxy captures with pagination
func (m *ProxyCapture) GetAll(
	ctx context.Context,
	options *ProxyCaptureOption,
) ([]database.ProxyCapture, bool, error) {
	var captures []database.ProxyCapture
	db := m.DB.Model(&database.ProxyCapture{})

	db, err := useQuery(db, database.PROXY_CAPTURE_TABLE, options.QueryArgs, proxyCaptureAllowedColumns...)
	if err != nil {
		return nil, false, err
	}

	dbRes := db.Find(&captures)
	if dbRes.Error != nil {
		return nil, false, dbRes.Error
	}

	hasNextPage, err := useHasNextPage(db, database.PROXY_CAPTURE_TABLE, options.QueryArgs, proxyCaptureAllowedColumns...)
	if err != nil {
		return nil, false, err
	}

	return captures, hasNextPage, nil
}

// GetByID gets a proxy capture by id
func (m *ProxyCapture) GetByID(
	ctx context.Context,
	id *uuid.UUID,
) (*database.ProxyCapture, error) {
	var capture database.ProxyCapture
	res := m.DB.Where("id = ?", id).First(&capture)
	if res.Error != nil {
		return nil, res.Error
	}
	return &capture, nil
}

// DeleteByID deletes a proxy capture by id
func (m *ProxyCapture) DeleteByID(
	ctx context.Context,
	id *uuid.UUID,
) error {
	res := m.DB.Delete(&database.ProxyCapture{}, id)
	return res.Error
}

// DeleteAll deletes all proxy captures
func (m *ProxyCapture) DeleteAll(ctx context.Context) error {
	res := m.DB.Where("1 = 1").Delete(&database.ProxyCapture{})
	return res.Error
}
