package hub

import (
	"log"

	"github.com/zdgeier/jamsync/gen/pb"
)

// Hub maintains active client state and message broadcasting
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan *pb.ChangeStreamMessage
	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan *pb.ChangeStreamMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (hub *Hub) Broadcast(message *pb.ChangeStreamMessage) {
	hub.broadcast <- message
}

func (hub *Hub) Run() {
	log.Printf("Registering clients\n")
	for {
		select {
		case client := <-hub.register:
			log.Printf("Registering client: %+v\n", client)
			hub.clients[client] = true
		case client := <-hub.unregister:
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				close(client.Send)
			}
		case message := <-hub.broadcast:
			for client := range hub.clients {
				if client.projectId == message.ProjectId && client.userId == message.UserId {
					client.Send <- message
				}
			}
		}
	}
}

func (hub *Hub) Register(projectId uint64, userId string) *Client {
	client := &Client{projectId, userId, hub, make(chan *pb.ChangeStreamMessage, 256)}
	client.hub.register <- client
	return client
}

type Client struct {
	projectId uint64
	userId    string
	hub       *Hub
	Send      chan *pb.ChangeStreamMessage
}
