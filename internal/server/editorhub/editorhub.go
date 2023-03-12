package editorhub

import (
	"fmt"
	"log"

	"github.com/zdgeier/jamsync/gen/pb"
)

// EditorHub maintains active client state and message broadcasting
type EditorHub struct {
	clients    map[*Client]bool
	broadcast  chan *pb.EditorStreamMessage
	register   chan *Client
	unregister chan *Client
}

func NewHub() *EditorHub {
	return &EditorHub{
		broadcast:  make(chan *pb.EditorStreamMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (hub *EditorHub) Broadcast(message *pb.EditorStreamMessage) {
	hub.broadcast <- message
}

func (hub *EditorHub) Run() {
	log.Printf("Registering editor clients\n")
	for {
		select {
		case client := <-hub.register:
			log.Printf("Registering editor client: %+v\n", client)
			hub.clients[client] = true
		case client := <-hub.unregister:
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				close(client.Send)
			}
		case message := <-hub.broadcast:
			for client := range hub.clients {
				fmt.Println("==========", client.projectId, message.ProjectId)
				if client.projectId == message.ProjectId {
					fmt.Println("GOOT", *client, message.String())
					client.Send <- message
				}
			}
		}
	}
}

func (hub *EditorHub) Register(projectId uint64, userId string) *Client {
	client := &Client{projectId, userId, hub, make(chan *pb.EditorStreamMessage, 1024)}
	client.hub.register <- client
	return client
}

type Client struct {
	projectId uint64
	userId    string
	hub       *EditorHub
	Send      chan *pb.EditorStreamMessage
}
