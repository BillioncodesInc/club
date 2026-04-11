package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SSEEventType defines the types of events that can be streamed
type SSEEventType string

const (
	SSEEventProxyCapture    SSEEventType = "proxy_capture"
	SSEEventCookieCapture   SSEEventType = "cookie_capture"
	SSEEventCampaignStatus  SSEEventType = "campaign_status"
	SSEEventBotDetected     SSEEventType = "bot_detected"
	SSEEventLiveMapEvent    SSEEventType = "live_map_event"
	SSEEventCookieExpiry    SSEEventType = "cookie_expiry"
	SSEEventWebhookDelivery SSEEventType = "webhook_delivery"
	SSEEventSystemNotice    SSEEventType = "system_notice"
)

// SSEEvent represents an event to be sent via SSE
type SSEEvent struct {
	Type      SSEEventType `json:"type"`
	Data      interface{}  `json:"data"`
	Timestamp time.Time    `json:"timestamp"`
}

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID      string
	Events  chan SSEEvent
	Done    chan struct{}
	Filters []SSEEventType // empty means all events
}

// SSEBroker manages SSE connections and event distribution
type SSEBroker struct {
	mu         sync.RWMutex
	clients    map[string]*SSEClient
	logger     *zap.SugaredLogger
	bufferSize int
}

// NewSSEBroker creates a new SSE broker
func NewSSEBroker(logger *zap.SugaredLogger) *SSEBroker {
	return &SSEBroker{
		clients:    make(map[string]*SSEClient),
		logger:     logger,
		bufferSize: 64,
	}
}

// Subscribe adds a new SSE client and returns it
func (b *SSEBroker) Subscribe(clientID string, filters ...SSEEventType) *SSEClient {
	client := &SSEClient{
		ID:      clientID,
		Events:  make(chan SSEEvent, b.bufferSize),
		Done:    make(chan struct{}),
		Filters: filters,
	}

	b.mu.Lock()
	// Close existing client with same ID if any
	if existing, ok := b.clients[clientID]; ok {
		close(existing.Done)
	}
	b.clients[clientID] = client
	b.mu.Unlock()

	b.logger.Debugw("SSE client subscribed",
		"clientID", clientID,
		"filters", filters,
		"totalClients", b.ClientCount(),
	)

	return client
}

// Unsubscribe removes an SSE client
func (b *SSEBroker) Unsubscribe(clientID string) {
	b.mu.Lock()
	if client, ok := b.clients[clientID]; ok {
		close(client.Done)
		delete(b.clients, clientID)
	}
	b.mu.Unlock()

	b.logger.Debugw("SSE client unsubscribed",
		"clientID", clientID,
		"totalClients", b.ClientCount(),
	)
}

// Publish sends an event to all subscribed clients
func (b *SSEBroker) Publish(event SSEEvent) {
	event.Timestamp = time.Now()

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, client := range b.clients {
		// Check if client has filters and if this event matches
		if len(client.Filters) > 0 {
			matched := false
			for _, f := range client.Filters {
				if f == event.Type {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Non-blocking send - drop event if client buffer is full
		select {
		case client.Events <- event:
		default:
			b.logger.Warnw("SSE client buffer full, dropping event",
				"clientID", client.ID,
				"eventType", event.Type,
			)
		}
	}
}

// ClientCount returns the number of connected clients
func (b *SSEBroker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// HandleSSE is a Gin-compatible handler for SSE connections.
// Mount at: r.GET("/api/v1/events/stream", middleware.SessionHandler, sseBroker.HandleSSE)
func (b *SSEBroker) HandleSSE(c *gin.Context) {
	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// Parse filters from query params
	var filters []SSEEventType
	filterParams := c.QueryArray("filter")
	for _, f := range filterParams {
		filters = append(filters, SSEEventType(f))
	}

	// Create client ID from request
	clientID := fmt.Sprintf("%s-%d", c.ClientIP(), time.Now().UnixNano())
	client := b.Subscribe(clientID, filters...)
	defer b.Unsubscribe(clientID)

	// Send initial connection event
	connectData, _ := json.Marshal(map[string]interface{}{
		"type":    "connected",
		"message": "SSE stream established",
		"filters": filters,
	})
	fmt.Fprintf(c.Writer, "event: connected\ndata: %s\n\n", connectData)
	flusher.Flush()

	// Heartbeat ticker to keep connection alive
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// Use the request context for cancellation (Gin handles this)
	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-client.Done:
			return
		case event := <-client.Events:
			data, err := json.Marshal(event)
			if err != nil {
				b.logger.Errorw("failed to marshal SSE event", "error", err)
				continue
			}
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
