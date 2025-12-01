package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/gorilla/websocket"

	"holler/shared"
)

var logOutput *widget.Entry

func logToUI(msg string) {
	logOutput.SetText(logOutput.Text + msg + "\n")
}

func handleMessagesFromServer(conn *websocket.Conn, authenticated *bool, onAuthenticated func()) {
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				logToUI("❌ Error reading: " + err.Error())
				return
			}

			var peek struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(data, &peek); err != nil {
				logToUI("❌ Unmarshal error: " + err.Error())
				continue
			}

			switch peek.Type {
			case "loginResponse":
				var resp sharedTypes.ServerResponse
				_ = json.Unmarshal(data, &resp)
				if resp.Success {
					*authenticated = true
					onAuthenticated()
				}
				logToUI("🔐 Login: " + resp.Message)
			case "serverResponse":
				var msg sharedTypes.Message
				_ = json.Unmarshal(data, &msg)
				logToUI("📨 Server: " + msg.Content)
			default:
				var msg sharedTypes.Message
				_ = json.Unmarshal(data, &msg)
				logToUI(fmt.Sprintf("👤 %s: %s", msg.Username, msg.Content))
			}
		}
	}()
}

func connectAndLogin(usernameEntry, passwordEntry *widget.Entry) *websocket.Conn {
	u := strings.TrimSpace(usernameEntry.Text)
	p := strings.TrimSpace(passwordEntry.Text)

	if u == "" || p == "" {
		logToUI("Username and password required")
		return nil
	}

	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
	if err != nil {
		logToUI("❌ Connection failed: " + err.Error())
		return nil
	}

	login := sharedTypes.Login{
		Type:     "login",
		Username: u,
		Password: p,
	}
	if err := conn.WriteJSON(login); err != nil {
		logToUI("❌ Send error on login: " + err.Error())
		return nil
	}

	return conn
}

func sendPost(usernameEntry *widget.Entry, messageEntry *widget.Entry, conn *websocket.Conn) {
	if conn == nil {
		logToUI("Not connected.")
		return
	}
	msg := sharedTypes.Message{
		Type:     "post",
		Username: usernameEntry.Text,
		Content:  messageEntry.Text,
	}
	_ = conn.WriteJSON(msg)
	messageEntry.SetText("")
}

func sendFollowRequest(usernameEntry *widget.Entry, targetUserEntry *widget.Entry, conn *websocket.Conn) {
	req := sharedTypes.FollowRequest{
		Type:               "makeFollowRequest",
		RequestingUserName: usernameEntry.Text,
		FollowedUserName:   targetUserEntry.Text,
	}
	_ = conn.WriteJSON(req)
}

func sendFriendRequest(usernameEntry *widget.Entry, targetUserEntry *widget.Entry, conn *websocket.Conn) {
	req := sharedTypes.FriendRequest{
		Type:               "makeFriendRequest",
		RequestingUserName: usernameEntry.Text,
		RequestedUserName:  targetUserEntry.Text,
	}
	_ = conn.WriteJSON(req)
}

func sendListFriendRequests(usernameEntry *widget.Entry, conn *websocket.Conn) {
	req := sharedTypes.ListFriendRequests{
		Type:     "listFriendRequests",
		Username: usernameEntry.Text,
	}
	_ = conn.WriteJSON(req)
}

func logout(conn *websocket.Conn, authenticated *bool, refreshUI func()) {
	req := sharedTypes.LogOut{
		Type:     "logOut",
	}
	_ = conn.WriteJSON(req)
	*authenticated = false
	refreshUI()
}

func buildUI(authenticated bool, conn **websocket.Conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry *widget.Entry, w fyne.Window, buildTarget string) fyne.CanvasObject {
	if !authenticated {
		return buildLoginUI(&authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w)
	}

	if buildTarget == "follow" {
		return buildSendFollowUI(&authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w)
	} 

	if buildTarget == "friend" {
		return buildSendFriendUI(&authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w)
	} 

	if buildTarget == "listFriends" {
		return buildListFriendsUI(&authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w)
	} 

	return buildSendMessageUI(&authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w)
	
}

func makeRefreshUI(
	authenticated *bool,
	conn **websocket.Conn,
	usernameEntry, passwordEntry, messageEntry, targetUserEntry *widget.Entry,
	w fyne.Window,
	view string,
) func() {
	return func() {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			w.SetContent(container.NewVScroll(
				buildUI(*authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, view),
			))
		}, false)
	}
}

func buildLoginUI(
	authenticated *bool,
	conn **websocket.Conn,
	usernameEntry, passwordEntry, messageEntry, targetUserEntry *widget.Entry,
	w fyne.Window,
) fyne.CanvasObject {
	
	onAuthenticated := func() {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			w.SetContent(container.NewVScroll(
				buildUI(*authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, ""),
			))
		}, false)
	}

	return container.NewVBox(
		widget.NewLabel("Username"),
		usernameEntry,
		widget.NewLabel("Password"),
		passwordEntry,
		widget.NewButton("Login & Connect", func() {
			*conn = connectAndLogin(usernameEntry, passwordEntry)
			if *conn != nil {
				handleMessagesFromServer(*conn, authenticated, onAuthenticated)
			}
		}),
		widget.NewLabel("Server Output"),
		logOutput,
	)
}

func buildSendMessageUI(
	authenticated *bool, 
	conn **websocket.Conn, usernameEntry, 
	passwordEntry, messageEntry, 
	targetUserEntry *widget.Entry, 
	w fyne.Window,
) fyne.CanvasObject {

	return container.NewVBox(		
		widget.NewLabel("Message"),
		messageEntry,
		widget.NewButton("Post", func() { sendPost(usernameEntry, passwordEntry, *conn) }),
		widget.NewLabel("Target Username (Follow/Friend)"),
		targetUserEntry,
		widget.NewButton("Send Follow Request", func() {
			makeRefreshUI(authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, "follow")()
		}),
		widget.NewButton("Send Friend Request", func() {
			makeRefreshUI(authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, "friend")()
		}),
		widget.NewButton("List Friend Requests", func() {
			makeRefreshUI(authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, "listFriends")()
		}),
		widget.NewButton("Logout", func() {
			logout(*conn, authenticated, func() { makeRefreshUI(authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, "logout")() })
		}),
		widget.NewLabel("Server Output"),
		logOutput,
	)
}

func buildSendFollowUI(
	authenticated *bool, 
	conn **websocket.Conn, usernameEntry, 
	passwordEntry, messageEntry, 
	targetUserEntry *widget.Entry, 
	w fyne.Window,
) fyne.CanvasObject {

	return container.NewVBox(		
		widget.NewLabel("Target Username to Follow"),
		targetUserEntry,
		widget.NewButton("Send Follow Request", func() { sendFollowRequest(usernameEntry, targetUserEntry, *conn) }),
		widget.NewButton("Logout", func() {
			logout(*conn, authenticated, func() { makeRefreshUI(authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, "logout")() })
		}),
		widget.NewLabel("Server Output"),
		logOutput,
	)
}

func buildSendFriendUI(
	authenticated *bool, 
	conn **websocket.Conn, usernameEntry, 
	passwordEntry, messageEntry, 
	targetUserEntry *widget.Entry, 
	w fyne.Window,
) fyne.CanvasObject {

	return container.NewVBox(		
		widget.NewLabel("Target Username to Friend"),
		targetUserEntry,
		widget.NewButton("Send Friend Request", func() { sendFriendRequest(usernameEntry, targetUserEntry, *conn) }),
		widget.NewButton("Logout", func() {
			logout(*conn, authenticated, func() { makeRefreshUI(authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, "logout")() })
		}),
		widget.NewLabel("Server Output"),
		logOutput,
	)
}

func buildListFriendsUI(
	authenticated *bool, 
	conn **websocket.Conn, usernameEntry, 
	passwordEntry, messageEntry, 
	targetUserEntry *widget.Entry, 
	w fyne.Window,
) fyne.CanvasObject {

	return container.NewVBox(		
		widget.NewButton("List Friends", func() { sendListFriendRequests(usernameEntry, *conn) }),
		widget.NewButton("Logout", func() {
			logout(*conn, authenticated, func() { makeRefreshUI(authenticated, conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, "logout")() })
		}),
		widget.NewLabel("Server Output"),
		logOutput,
	)
}

func main() {
	a := app.New()
	w := a.NewWindow("Holler Client")
	w.Resize(fyne.NewSize(600, 500))

	var conn *websocket.Conn
	usernameEntry := widget.NewEntry()
	passwordEntry := widget.NewPasswordEntry()
	messageEntry := widget.NewEntry()
	targetUserEntry := widget.NewEntry()

	logOutput = widget.NewMultiLineEntry()
	logOutput.Wrapping = fyne.TextWrapWord
	logOutput.SetMinRowsVisible(10)
	// logOutput.Disable()

	authenticated := false

	ui := buildUI(authenticated, &conn, usernameEntry, passwordEntry, messageEntry, targetUserEntry, w, "")
	scrollable := container.NewVScroll(ui)
	w.SetContent(scrollable)
	w.ShowAndRun()
}
