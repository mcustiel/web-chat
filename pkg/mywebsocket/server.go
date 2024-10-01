package mywebsocket

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Server struct {
	clients map[string]*Client
}

func NewServer() *Server {
	return &Server{ clients: make(map[string]*Client) }
}

func (s *Server) register(c *Client) {
	id := c.Id()
	log.Println("Registering client", id)
	s.clients[id] = c
}

func (s *Server) unregister(c *Client) {
	id := c.Id()
	log.Println("Unregistering client", id)

	if _, ok := s.clients[id]; ok {
		delete(s.clients, id)
		log.Println("Successfully unregistered client")
	}
}

func (s *Server) broadcast(message []byte) {
	log.Println("Received broadcast message:", string(message))
	for id, client := range s.clients {
		log.Println("Sending message to client:", id)
		client.Send(message)
	}
}

func (s *Server) ServeWs(w http.ResponseWriter, r *http.Request) {
	log.Println("ServeWS starting")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client, err := NewClient(fmt.Sprintf("id-%d", len(s.clients)+1), conn)
	if err != nil {
		log.Println(err)
		return
	}
	s.register(client)
	go client.Run(s)
}
