package mywebsocket

import (
	"github.com/gorilla/websocket"

	"bytes"
	"log"
	"time"
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

type Client struct {
	id string
	// The websocket connection.
	conn *websocket.Conn
}

func NewClient(id string, conn *websocket.Conn) (*Client, error) {
	return &Client{
		id:   id,
		conn: conn,
	}, nil
}

func (c *Client) Id() string {
	return c.id
}

func (c *Client) Run(s *Server) {
	go c.readPump(s)
	go c.keepAlive(s)
}

func (c *Client) Send(msg []byte) {
	log.Println("Sending message", string(msg))
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	// if !ok {
	// 	log.Println("The hub closed the channel")
	// 	// The hub closed the channel.
	// 	c.conn.WriteMessage(websocket.CloseMessage, []byte{})
	// 	return
	// }

	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		log.Println("Error getting next writer", err)
		return
	}
	w.Write(msg)
	log.Println("Wrote message:", string(msg))

	// Add queued chat messages to the current websocket message.
	// n := len(c.send)
	// for i := 0; i < n; i++ {
	// 	msg := <-c.send
	// 	log.Println("Writting queued message:", string(msg))
	// 	w.Write(newline)
	// 	w.Write(msg)
	// }

	if err := w.Close(); err != nil {
		log.Print("Error closing writer", err)
		return
	}
	log.Print("Closed writer")
}

func (c *Client) Close(s *Server) {
		s.unregister(c)
		c.conn.Close()
}

func (c *Client) keepAlive(s *Server) {
	log.Println("keepAlive starting")
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		log.Println("keepAlive exiting")
		ticker.Stop()
		c.Close(s)
	}()

	for {
		select {
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump(server *Server) {
	log.Println("readPump starting")
	defer func() {
		log.Println("readPump exiting")
		c.Close(server)
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
		server.broadcast(message)
	}
	log.Println("Exiting readPump")
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
