package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"inkdrop/config"
	"inkdrop/event"
	"inkdrop/model"

	"github.com/gorilla/websocket"
)

// Client represents a connected WebSocket client.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID int64
	dropID string // empty means all drops
}

// Hub manages WebSocket connections and message broadcasting.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

// Run starts the hub's event processing loop.
func (h *Hub) Run() {
	// Subscribe to global event dispatcher
	event.GlobalDispatcher.Subscribe(func(e event.Event) {
		data, err := json.Marshal(e)
		if err != nil {
			return
		}
		h.BroadcastToDrop(e.DropID, data)
	})
}

// BroadcastToDrop sends a message to all clients subscribed to a drop.
func (h *Hub) BroadcastToDrop(dropID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.dropID == "" || client.dropID == dropID {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = true
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// HandleWebSocket handles WebSocket upgrade requests.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate via session
	ss := config.GetSessionManager()
	isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
	userName := ss.GetString(r.Context(), "name")
	if !isLoggedIn || userName == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user ID
	db := config.GetDB()
	user, err := model.GetUserByName(userName, db)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Get drop ID from query param
	dropID := r.URL.Query().Get("drop_id")

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: user.ID,
		dropID: dropID,
	}
	h.Register(client)

	// Start write pump
	go client.writePump()

	// Read pump (blocks until disconnect)
	client.readPump()

	h.Unregister(client)
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer c.conn.Close()
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg map[string]interface{}
		if err := c.conn.ReadJSON(&msg); err != nil {
			break
		}
		// Handle incoming messages (e.g., ping, cursor position, etc.)
		if msgType, ok := msg["type"].(string); ok {
			switch msgType {
			case "ping":
				c.conn.WriteJSON(map[string]string{"type": "pong"})
			}
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// GlobalHub is the application-wide WebSocket hub.
var GlobalHub = NewHub()

func init() {
	go GlobalHub.Run()
	log.Println("WebSocket hub initialized")
}
