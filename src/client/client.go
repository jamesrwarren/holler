package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"holler/shared"
)

func handleMessagesFromServer (conn *websocket.Conn) {
	// Goroutine to listen for incoming messages
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Println("❌ Error reading from server:", err)
				return
			}
	
			// Peek inside to get the "type"
			var peek struct {
				Type string `json:"type"`
			}
			err = json.Unmarshal(data, &peek)
			if err != nil {
				log.Println("❌ Error unmarshaling type:", err)
				continue
			}
	
			switch peek.Type {
				case "loginResponse":
					var resp sharedTypes.ServerResponse
					err = json.Unmarshal(data, &resp)
					if err != nil {
						log.Println("❌ Error unmarshaling response:", err)
						continue
					}
					if resp.Success {
						fmt.Println("🎉 Login successful:", resp.Message)
					} else {
						fmt.Println("❌ Login failed:", resp.Message)
					}
				case "serverResponse":
					var msg sharedTypes.Message
					err = json.Unmarshal(data, &msg)
					if err != nil {
						log.Println("❌ Error unmarshaling message:", err)
						continue
					}
					fmt.Printf("📨 Response from Server: %s\n", msg.Content)					
				default:
					var msg sharedTypes.Message
					err = json.Unmarshal(data, &msg)
					if err != nil {
						log.Println("❌ Error unmarshaling message:", err)
						continue
					}
					fmt.Printf("📨 From %s: %s\n", msg.Username, msg.Content)
				}
		}
	}()
}

func sendLogin(conn *websocket.Conn, reader *bufio.Reader, typeOfMsg string, username string) (bool) {
	fmt.Print("Enter your password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	login := sharedTypes.Login{
		Type: typeOfMsg,
		Username: username,
		Password: password,
	}

	err := conn.WriteJSON(login)
	if err != nil {
		log.Println("❌ Send error on login:", err)
		return false
	}

	return true
}

func sendPost(conn *websocket.Conn, reader *bufio.Reader, typeOfMsg string, username string) (bool) {
	fmt.Print("Enter your message: ")
	message, _ := reader.ReadString('\n')
	message = strings.TrimSpace(message)
	msg := sharedTypes.Message{
		Type: typeOfMsg,
		Username: username,
		Content: message,
	}
	err := conn.WriteJSON(msg)
	if err != nil {
		log.Println("❌ Send error on message:", err)
		return false
	}
	return true
}

func sendFollowRequest(conn *websocket.Conn, reader *bufio.Reader, typeOfMsg string, username string) (bool) {
	fmt.Print("Enter username to follow: ")
	usernameToFollow, _ := reader.ReadString('\n')
	usernameToFollow = strings.TrimSpace(usernameToFollow)
	followRequest := sharedTypes.FollowRequest{
		Type: typeOfMsg,
		RequestingUserName: username,
		FollowedUserName: usernameToFollow,
	}
	err := conn.WriteJSON(followRequest)
	if err != nil {
		log.Println("❌ Send error on message:", err)
		return false
	}
	return true
}

func sendFriendRequest(conn *websocket.Conn, reader *bufio.Reader, typeOfMsg string, username string) (bool) {
	fmt.Print("Enter username to follow: ")
	usernameToFriend, _ := reader.ReadString('\n')
	usernameToFriend = strings.TrimSpace(usernameToFriend)
	friendRequest := sharedTypes.FriendRequest{
		Type: typeOfMsg,
		RequestingUserName: username,
		RequestedUserName: usernameToFriend,
	}
	err := conn.WriteJSON(friendRequest)
	if err != nil {
		log.Println("❌ Send error on message:", err)
		return false
	}
	return true
}

func sendListFriendRequests(conn *websocket.Conn, reader *bufio.Reader, typeOfMsg string, username string) (bool) {
	listFriendsRequest := sharedTypes.ListFriendRequests{
		Type: typeOfMsg,
		Username: username,
	}
	err := conn.WriteJSON(listFriendsRequest)
	if err != nil {
		log.Println("❌ Send error on message:", err)
		return false
	}
	return true
}

func handleSendingMessages (conn *websocket.Conn, reader *bufio.Reader, username string) {
	for {
		content := "";
		fmt.Print("Type: ")
		typeOfMsg, _ := reader.ReadString('\n')
		typeOfMsg = strings.TrimSpace(typeOfMsg)


		switch typeOfMsg {
			case "login":
				sendLogin(conn, reader, typeOfMsg, username) 
			case "post":
				sendPost(conn, reader, typeOfMsg, username)
			case "makeFollowRequest":
				sendFollowRequest(conn, reader, typeOfMsg, username)
			case "makeFriendRequest":
				sendFriendRequest(conn, reader, typeOfMsg, username)
			case "listFriendRequests":
				sendListFriendRequests(conn, reader, typeOfMsg, username)
			default:
				log.Printf("Unknown Type: %s", typeOfMsg)
		}

		if content == "" || typeOfMsg == "" {
			continue
		}

		msg := sharedTypes.Message{
			Username: username,
			Type: typeOfMsg,
			Content: content,
		}

		log.Println("Message to send: ", msg)

		err := conn.WriteJSON(msg)
		if err != nil {
			log.Println("❌ Send error:", err)
			break
		}
	}
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

	handleMessagesFromServer(conn)

	handleSendingMessages(conn, reader, username)
}
