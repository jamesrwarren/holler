package sharedTypes


type Message struct {
	Type string `json:"type"`
	Username string `json:"username"`
	Content string `json:"content"`
}

type Login struct {
	Type string `json:"type"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Response struct {
	Type    string `json:"type"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type FollowRequest struct {
	Type    string `json:"type"`
	RequestingUserName string `json:"requestor_id"`
	FollowedUserName string   `json:"followed_id"`
}

type FriendRequest struct {
	Type    string `json:"type"`
	RequestingUserName string `json:"requestor_id"`
	RequestedUserName string   `json:"followed_id"`
}

type ListFriendRequests struct {
	Type    string `json:"type"`
	Username string `json:"username"`
}
