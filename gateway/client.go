package gateway

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024 // 512KB
)

// Client represents a connected WebSocket client.
type Client struct {
	ID       string
	conn     *websocket.Conn
	gateway  *Gateway
	remoteIP string // Source IP of the connection, used to key auth penalties
	send     chan *Message
	done     chan struct{}
	once     sync.Once
	metadata map[string]interface{}
	mu       sync.RWMutex
}

// newClient creates a new client.
func newClient(conn *websocket.Conn, gateway *Gateway) *Client {
	return &Client{
		ID:       uuid.New().String(),
		conn:     conn,
		gateway:  gateway,
		remoteIP: connRemoteIP(conn),
		send:     make(chan *Message, 256),
		done:     make(chan struct{}),
		metadata: make(map[string]interface{}),
	}
}

// connRemoteIP extracts the source IP from a WebSocket connection's remote
// address, without the port so reconnects key to the same penalty state.
func connRemoteIP(conn *websocket.Conn) string {
	if conn == nil {
		return ""
	}
	addr := conn.RemoteAddr()
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

// RemoteIP returns the client's source IP. Falls back to the client ID when
// the connection carries no address (e.g. in tests), so penalty keying still
// distinguishes callers.
func (c *Client) RemoteIP() string {
	if c.remoteIP == "" {
		return c.ID
	}
	return c.remoteIP
}

// Send queues a message to be sent to the client.
func (c *Client) Send(msg *Message) {
	select {
	case c.send <- msg:
	case <-c.done:
	default:
		// Channel full, drop message
		c.gateway.logger.Warn("message dropped, send buffer full", "client", c.ID)
	}
}

// Close closes the client connection.
func (c *Client) Close() {
	c.once.Do(func() {
		close(c.done)
		c.conn.Close()
		c.gateway.unregisterClient(c)
	})
}

// SetMetadata sets a metadata value.
func (c *Client) SetMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata gets a metadata value.
func (c *Client) GetMetadata(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.metadata[key]
	return v, ok
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer c.Close()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.gateway.logger.Error("websocket read error", "client", c.ID, "error", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			c.gateway.logger.Error("message decode error", "client", c.ID, "error", err)
			continue
		}

		// Handle message
		if c.gateway.onMessage != nil {
			ctx := context.Background()
			response, err := c.gateway.onMessage(ctx, c, &msg)
			if err != nil {
				c.gateway.logger.Error("message handler error", "client", c.ID, "error", err)
				c.Send(&Message{
					Type:  MessageTypeError,
					Error: err.Error(),
				})
				continue
			}
			if response != nil {
				c.Send(response)
			}
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := json.Marshal(msg)
			if err != nil {
				c.gateway.logger.Error("message encode error", "client", c.ID, "error", err)
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.gateway.logger.Error("websocket write error", "client", c.ID, "error", err)
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}
