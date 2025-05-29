package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"database/sql"

	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"

	"holler/shared"
)

var db *sql.DB

type BroadcastObject struct {
	ClientDetails Client
	Data []byte
}

type Client struct {
	Username string
	Authenticated bool
	Conn     *websocket.Conn
}

// Used for peeking at the type
var peek struct {
	Type string `json:"type"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}


var clients = make(map[*websocket.Conn]Client)
var broadcast = make(chan BroadcastObject)

func main() {
	var err error
	connStr := "postgres://holler:holler@localhost:5432/holler?sslmode=disable" 
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("❌ Failed to connect to DB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ DB ping failed: %v", err)
	}

	setupHollerEnvironment()

	// Checks for auth in here and passes through requests to handleRequests
	http.HandleFunc("/ws", handleClientConnection)
	// main request handler for authneticated users!
	go handleRequests()

	fmt.Println("✅ Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func setupHollerEnvironment() {
	query := `
	CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,
		sender TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("❌ Failed to create messages table: %v", err)
	}

	query = `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username TEXT NOT NULL,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		profile TEXT NOT NULL,
		logged_in BOOL,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("❌ Failed to create users table: %v", err)
	}

	query = `
	CREATE TABLE IF NOT EXISTS follow (
		id SERIAL PRIMARY KEY,
		poster_user_id INT NOT NULL,
		follower_user_id INT NOT NULL,
		friend_requested BOOL,
		friend_accepted BOOL
	);`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("❌ Failed to create follow table: %v", err)
	}
}

func handleClientConnection(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading connection to websocket:", err)
		return
	}
	defer ws.Close()

	// We don't currently know who is logging in but we have a connection!
	clients[ws] = Client{Username: "", Authenticated: false, Conn: ws}

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			log.Printf("❌ Disconnecting - Read error from %s: %v", clients[ws].Username, err)
			delete(clients, ws)
			break
		}

		client := clients[ws]

		err = json.Unmarshal(data, &peek)
		if err != nil {
			log.Println("❌ Error unmarshaling type:", err)
			continue
		}

		switch peek.Type {
			case "login":
				var login sharedTypes.Login
				err = json.Unmarshal(data, &login)
				if err != nil {
					log.Println("❌ Error unmarshaling login:", err)
					continue
				}	
				client.Username = login.Username
				loginResponse := handleLogin(login.Username, login.Password)
				
				if loginResponse.Success {
					client.Authenticated = true
					clients[ws] = client
					ws.WriteJSON(loginResponse)
				} else {
					ws.WriteJSON(loginResponse)
					delete(clients, ws)
					break
				}
				clients[ws] = client
			default:
				// If they try and do anything without being logged in remove them and their connection
				if (!client.Authenticated) {
					log.Printf("❌ Client not authenticated %s: %v", client.Username, err)
					delete(clients, ws)
					break
				}
				// Otherwise broadcast their action for handling handleRequests loop
				broadcastObject := BroadcastObject{ClientDetails: clients[ws], Data: data}
				broadcast <- broadcastObject
				log.Printf("%s posted a %s request as authenticated user", client.Username, peek.Type)
		}
	}
}

func handleLogin(username string, password string) (sharedTypes.Response) {
	log.Printf("🦾 Trying to login with %s: %s", username, password)
	if password == "password" {
		log.Printf("🐝 Login Success for %s", username)

		loginResponse := sharedTypes.Response{
			Type:    "loginResponse",
			Success: true,
			Message: "✅ Login successful!",
		}
		return loginResponse
	} 

	log.Printf("❌ Login Unsuccessful %s: %v", username, password)
	loginResponse := sharedTypes.Response{
			Type:    "loginResponse",
			Success: false,
			Message: "❌ Login unsuccessful!",
	}

	return loginResponse
}

// This is the main function that splits out what type of request we received and what to do with it
func handleRequests() {
	for {
		broadcastObject := <-broadcast
		data := broadcastObject.Data;
		client := broadcastObject.ClientDetails;
		
		log.Printf("Client Details In Message: %s, %s", client.Username, strconv.FormatBool(client.Authenticated))

		err := json.Unmarshal(data, &peek)
		if err != nil {
			log.Println("❌ Error unmarshaling type:", err)
			continue
		}

		switch peek.Type {
			case "post":
				handlePost(data);
			case "makeFollowRequest":
				handleFollowRequest(data, client)
			case "makeFriendRequest":
				handleFriendRequest(data, client)
			case "listFriendRequests":
				listFriendRequests(data, client)
			// case "acceptFriendRequest":
			// 	handleFollowRequest(data)
			default:
				log.Printf("Unknown Type: %s", peek.Type)
		}
	}
}

func handlePost(data []byte) {
	var msg sharedTypes.Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Println("❌ Error unmarshaling message:", err)
		return
	}	

	// Insert message into DB for now. We're not doing anything with this yet...
	_, err = db.Exec("INSERT INTO messages (sender, content) VALUES ($1, $2)", msg.Username, msg.Content)
	if err != nil {
		log.Printf("⚠️ DB insert error: %v", err)
	}

	// Post to everyone for now
	for _, client := range clients {
		log.Printf("⏱️ Trying to send message to %s from %s", client.Username, msg.Username)	
		if (msg.Username != client.Username) {
			err := client.Conn.WriteJSON(msg)
			if err != nil {
				log.Printf("⚠️ Send error from %s: %v", msg.Username, err)
				client.Conn.Close()
				delete(clients, client.Conn)
			}
			log.Printf("✅ Sent message to %s from %s", client.Username, msg.Username)
		}
	}
}

func getRequestingAndRequestedIDs(requestingUserName string, requestedUserName string) (int, int) {
	var requestingUserID int
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", requestingUserName).Scan(&requestingUserID)
	if err != nil {
		log.Printf("⚠️ DB select error: %v", err)
	}

	var FollowedUserID int
	err = db.QueryRow("SELECT id FROM users WHERE username = $1", requestedUserName).Scan(&FollowedUserID)
	if err != nil {
		log.Printf("⚠️ DB select error: %v", err)
	}

	return requestingUserID, FollowedUserID
}

func handleFollowRequest(data []byte, client Client) {
	var follow sharedTypes.FollowRequest
	err := json.Unmarshal(data, &follow)
	if err != nil {
		log.Println("❌ Error unmarshaling follow request:", err)
		return
	}
	
	followedUserID, requestingUserID := getRequestingAndRequestedIDs(follow.RequestingUserName, follow.FollowedUserName)

	// Insert follow request
	_, err = db.Exec("INSERT INTO follow (poster_user_id, follower_user_id, friend_requested, friend_accepted) VALUES ($1, $2, $3, $4)", followedUserID, requestingUserID, false, false)
	if err != nil {
		log.Printf("⚠️ DB insert error: %v", err)
	}

	response := sharedTypes.Message{
		Type:    "serverResponse",
		Username: follow.RequestingUserName,
		Content: fmt.Sprintf("✅ request sent to %s!", follow.FollowedUserName),
	}
	err = client.Conn.WriteJSON(response)
	if err != nil {
		log.Printf("⚠️ Send error from %s: %v", response.Username, err)
		client.Conn.Close()
		delete(clients, client.Conn)
	}

	log.Printf("⏱️ Notifying %s of follow by %s", follow.FollowedUserName, client.Username)	
	
	msg := sharedTypes.Message{
		Type:    "post",
		Username: follow.RequestingUserName,
		Content: fmt.Sprintf("✅ %s is following you!", follow.RequestingUserName),
	}

	for _, client := range clients {
		log.Printf("⏱️ Trying to send message to %s from %s", client.Username, msg.Username)	
		if (follow.FollowedUserName == client.Username) {
			err := client.Conn.WriteJSON(msg)
			if err != nil {
				log.Printf("⚠️ Send error from %s: %v", msg.Username, err)
				client.Conn.Close()
				delete(clients, client.Conn)
			}
			log.Printf("✅ Sent message to %s from %s", client.Username, msg.Username)
		}
	}
}

func handleFriendRequest(data []byte, client Client) {
	var friend sharedTypes.FriendRequest
	err := json.Unmarshal(data, &friend)
	if err != nil {
		log.Println("❌ Error unmarshaling follow request:", err)
		return
	}
	
	followedUserID, requestingUserID := getRequestingAndRequestedIDs(friend.RequestingUserName, friend.RequestedUserName)

	// Insert follow request
	_, err = db.Exec("INSERT INTO follow (poster_user_id, follower_user_id, friend_requested, friend_accepted) VALUES ($1, $2, $3, $4)", followedUserID, requestingUserID, true, false)
	if err != nil {
		log.Printf("⚠️ DB insert error: %v", err)
	}

	for _, client := range clients {
		log.Printf("⏱️ Notifying %s of friend request by %s", friend.RequestedUserName, friend.RequestingUserName)	
		if (friend.RequestedUserName == client.Username) {
			msg := sharedTypes.Message{
				Type:    "post",
				Username: friend.RequestedUserName,
				Content: fmt.Sprintf("✅ %s sent you a friend request!", friend.RequestedUserName),
			}
			err := client.Conn.WriteJSON(msg)
			if err != nil {
				log.Printf("⚠️ Send error from %s: %v", msg.Username, err)
				client.Conn.Close()
				delete(clients, client.Conn)
			}
			log.Printf("✅ Sent friend request to %s from %s", friend.RequestedUserName, friend.RequestingUserName)
		}
	}
}

func listFriendRequests(data []byte, client Client) {
	var listFriendRequests sharedTypes.ListFriendRequests
	err := json.Unmarshal(data, &listFriendRequests)
	if err != nil {
		log.Println("❌ Error unmarshaling listFriendRequests:", err)
		return
	}
	
	var RequestedUserID int
	err = db.QueryRow("SELECT id FROM users WHERE username = $1", listFriendRequests.Username).Scan(&RequestedUserID)
	if err != nil {
		log.Printf("⚠️ DB select error: %v", err)
		return // Optional: exit early if this fails
	}
	
	var friendRequestUsernames []string
	
	rows, err := db.Query(`
		SELECT u.username
		FROM users u
		INNER JOIN follow f ON f.follower_user_id = u.id
		WHERE f.friend_requested = true
		AND f.poster_user_id = $1`, RequestedUserID)
	if err != nil {
		log.Printf("⚠️ DB select error: %v", err)
		return
	}
	defer rows.Close()
	
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			log.Printf("⚠️ Row scan error: %v", err)
			continue
		}
		friendRequestUsernames = append(friendRequestUsernames, username)
	}
	
	if err = rows.Err(); err != nil {
		log.Printf("⚠️ Rows iteration error: %v", err)
	}

	response := sharedTypes.Message{
		Type:    "serverResponse",
		Username: listFriendRequests.Username,
		Content: strings.Join(friendRequestUsernames[:],","),
	}
	err = client.Conn.WriteJSON(response)
	if err != nil {
		log.Printf("⚠️ Send error from %s: %v", response.Username, err)
		client.Conn.Close()
		delete(clients, client.Conn)
	}
}

