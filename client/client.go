package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

type Message struct {
	To      string `json:"to"`
	From    string `json:"from"`
	Content string `json:"content"`
}

func main() {
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
	if err != nil {
		log.Fatal("❌ Connection failed:", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(os.Stdin)

	// Ask for username
	fmt.Print("Enter your username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	// Send username as the first message
	err = conn.WriteMessage(websocket.TextMessage, []byte(username))
	if err != nil {
		log.Fatal("❌ Failed to send username:", err)
	}

	fmt.Println("✅ Connected. You can now send messages.")
	fmt.Println("Format: enter recipient username and message.")

	// Goroutine to listen for incoming messages
	go func() {
		for {
			var msg Message
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Println("❌ Error reading from server:", err)
				return
			}
			fmt.Printf("📨 From %s: %s\n", msg.From, msg.Content)
		}
	}()

	// Main loop: send messages
	for {
		fmt.Print("To: ")
		to, _ := reader.ReadString('\n')
		to = strings.TrimSpace(to)

		fmt.Print("Message: ")
		content, _ := reader.ReadString('\n')
		content = strings.TrimSpace(content)

		if to == "" || content == "" {
			continue
		}

		msg := Message{
			To:      to,
			Content: content,
		}

		err := conn.WriteJSON(msg)
		if err != nil {
			log.Println("❌ Send error:", err)
			break
		}
	}
}
