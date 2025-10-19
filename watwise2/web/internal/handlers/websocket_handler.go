package handlers

import (
	"log"
	"sync"
	"time"
	"wattwise/internal/database"
	"wattwise/internal/models"

	"github.com/gofiber/websocket/v2"
)

type WebSocketHandler struct {
	db            *database.IoTDB
	clients       map[*websocket.Conn]bool
	clientsMutex  sync.RWMutex
	broadcast     chan interface{}
	register      chan *websocket.Conn
	unregister    chan *websocket.Conn
}

func NewWebSocketHandler(db *database.IoTDB) *WebSocketHandler {
	handler := &WebSocketHandler{
		db:         db,
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan interface{}, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
	
	// Start hub untuk manage connections dan broadcasting
	go handler.runHub()
	
	return handler
}

// runHub manages WebSocket connections dan broadcasting
func (h *WebSocketHandler) runHub() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case conn := <-h.register:
			h.clientsMutex.Lock()
			h.clients[conn] = true
			h.clientsMutex.Unlock()
			log.Printf("🔌 Client registered. Total clients: %d", len(h.clients))
			
		case conn := <-h.unregister:
			h.clientsMutex.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.clientsMutex.Unlock()
			log.Printf("🔌 Client unregistered. Total clients: %d", len(h.clients))
			
		case message := <-h.broadcast:
			h.clientsMutex.RLock()
			clientCount := len(h.clients)
			for conn := range h.clients {
				err := conn.WriteJSON(message)
				if err != nil {
					log.Printf("❌ Error sending to client: %v", err)
					go func(c *websocket.Conn) {
						h.unregister <- c
					}(conn)
				}
			}
			h.clientsMutex.RUnlock()
			
			if clientCount > 0 {
				log.Printf("✅ Broadcasted to %d client(s)", clientCount)
			}
			
		case <-ticker.C:
			// Periodic status log (tidak fetch data lagi)
			h.clientsMutex.RLock()
			clientCount := len(h.clients)
			h.clientsMutex.RUnlock()
			
			if clientCount > 0 {
				log.Printf("📊 Active WebSocket clients: %d", clientCount)
			}
		}
	}
}

// BroadcastRealtimeData broadcasts data dari MQTT ke semua clients
func (h *WebSocketHandler) BroadcastRealtimeData(data models.RealtimeData) {
	h.clientsMutex.RLock()
	clientCount := len(h.clients)
	h.clientsMutex.RUnlock()
	
	if clientCount == 0 {
		log.Printf("⚠️ No WebSocket clients connected, skipping broadcast")
		return
	}
	
	select {
	case h.broadcast <- data:
		log.Printf("📤 Broadcasting realtime data: %s to %d client(s)", data.DeviceID, clientCount)
	default:
		log.Printf("⚠️ Broadcast channel full, dropping message")
	}
}

// BroadcastAlert broadcasts alert ke semua clients
func (h *WebSocketHandler) BroadcastAlert(alert models.AlertData) {
	h.clientsMutex.RLock()
	clientCount := len(h.clients)
	h.clientsMutex.RUnlock()
	
	if clientCount == 0 {
		return
	}
	
	select {
	case h.broadcast <- alert:
		log.Printf("⚠️ Broadcasting alert: %s - %s to %d client(s)", alert.AlertType, alert.Message, clientCount)
	default:
		log.Printf("⚠️ Broadcast channel full, dropping alert")
	}
}

// HandleConnection handles individual WebSocket connections
func (h *WebSocketHandler) HandleConnection(c *websocket.Conn) {
	clientID := c.RemoteAddr().String()
	log.Printf("📡 WebSocket client connected: %s", clientID)

	// Register client
	h.register <- c

	defer func() {
		h.unregister <- c
		log.Printf("📡 WebSocket client disconnected: %s", clientID)
	}()

	// Send welcome message (bukan dummy data)
	welcomeMsg := map[string]interface{}{
		"type":    "connected",
		"message": "WebSocket connected successfully",
		"server":  "Wattwise Energy Monitor",
		"time":    time.Now().Format(time.RFC3339),
	}
	
	err := c.WriteJSON(welcomeMsg)
	if err != nil {
		log.Printf("❌ Failed to send welcome message: %v", err)
		return
	}
	
	log.Printf("✅ Welcome message sent to %s", clientID)

	// Listen for messages from client (optional - untuk control)
	for {
		messageType, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("⚠️ WebSocket error from %s: %v", clientID, err)
			}
			break
		}

		if messageType == websocket.TextMessage {
			log.Printf("📨 Received from %s: %s", clientID, string(message))
			// Handle client commands here if needed
			// h.handleClientCommand(c, message)
		}
	}
}

// GetConnectedClients returns jumlah clients yang terkoneksi
func (h *WebSocketHandler) GetConnectedClients() int {
	h.clientsMutex.RLock()
	defer h.clientsMutex.RUnlock()
	return len(h.clients)
}