package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type Client struct {
	Username string
	Conn     *websocket.Conn
}

type Message struct {
	To      string `json:"to"`
	From    string `json:"from"`
	Content string `json:"content"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]Client)
var broadcast = make(chan Message)

func main() {
	http.HandleFunc("/ws", handleConnections)
	go handleMessages()

	fmt.Println("✅ Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer ws.Close()

	// Get username from the first message
	_, msg, err := ws.ReadMessage()
	if err != nil {
		log.Println("❌ Failed to read username:", err)
		return
	}
	username := string(msg)
	log.Printf("👤 %s connected", username)

	clients[ws] = Client{Username: username, Conn: ws}

	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("❌ Read error from %s: %v", username, err)
			delete(clients, ws)
			break
		}

		// Attach sender's username
		msg.From = clients[ws].Username
		broadcast <- msg
	}
}

func handleMessages() {
	for {
		msg := <-broadcast

		// Find recipient by username
		var found bool
		for _, client := range clients {
			if client.Username == msg.To {
				err := client.Conn.WriteJSON(msg)
				if err != nil {
					log.Printf("⚠️ Send error to %s: %v", msg.To, err)
					client.Conn.Close()
					delete(clients, client.Conn)
				}
				found = true
				break
			}
		}
		if !found {
			log.Printf("📭 User %s not connected", msg.To)
		}
	}
}
