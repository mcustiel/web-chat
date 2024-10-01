package websocket

import (
	"bytes"
	"log"
	"net/http"
	"time"

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
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	log.Println("readPump starting")
	defer func() {
		log.Println("readPump exiting")
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Error receiving message: ", err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		log.Println("Received message: ", message, messageTypeToString(messageType), ". broadcasting...")
		c.hub.broadcast <- message
	}
}

func messageTypeToString(messageType int) string {
	switch messageType {
	case websocket.PingMessage:
		return "ping"
	case websocket.PongMessage:
		return "pong"
	case websocket.CloseMessage:
		return "close"
	case websocket.BinaryMessage:
		return "binary"
	case websocket.TextMessage:
		return "text"
	}
	return "unknown"
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	log.Println("writePump starting")
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		log.Println("writePump exiting")
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			log.Println("Message sent")
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				log.Println("The hub closed the channel")
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Println("Error getting next writer", err)
				return
			}
			w.Write(message)
			log.Println("Wrote message:", string(message))

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				msg := <-c.send
				log.Println("Writting queued message:", string(msg))
				w.Write(newline)
				w.Write(msg)
			}

			if err := w.Close(); err != nil {
				log.Print("Error closing writer", err)
				return
			}
			log.Print("Closed writer")
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	log.Println("ServeWS starting")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
	log.Println("ServeWS ending")
}
