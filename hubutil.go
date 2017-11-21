package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// A unique ID.
type UUID string

// Represents a stored message sent by either a user or the server.
type Message struct {
	Text       string
	Type       string
	Room       string
	DateTime   string
	TargetUUID UUID
	Error      string
	Username   string
}

// Marshal message into string.
func (m *Message) marshalRequest() (string, error) {
	str, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(str), nil
}

// Marshall message into string and pipe to output channel.
func (m *Message) marshalRequestToChan(out chan string) {
	str, err := json.Marshal(m)
	if err != nil {
		out <- err.Error()
		return
	}
	out <- string(str)
}

// Unmarshal string into message.
func (m *Message) unmarshalRequest(request string) {
	json.Unmarshal([]byte(request), m)
}

var (
	// all chat rooms (key is room name, value is user object)
	rooms = make(map[string]*room)
	// all connected clients (key is UUID, value is user object)
	users = make(map[UUID]*user)
)

// A chat room containing users and all room messages.
type room struct {
	userIDs  map[UUID]bool
	messages []Message
	creator  UUID
}

// Create & initialise room.
func NewRoom(name string, creator UUID) (*room, error) {
	// check if room name is already taken
	if _, ok := rooms[name]; ok {
		return nil, fmt.Errorf("a room by that name already exists")
	}

	// add new room to rooms map
	r := &room{creator: creator, userIDs: make(map[UUID]bool)}
	rooms[name] = r

	return r, nil
}

// Add user to chat room.
func (r *room) AddUser(userID UUID) {
	r.userIDs[userID] = true
}

// Remove user from chat room.
func (r *room) RemoveUser(userID UUID) {
	delete(r.userIDs, userID)
}

// Check if user is subscribed to a room.
func (r *room) IsUserSubscribed(userID UUID) bool {
	_, ok := r.userIDs[userID]
	return ok
}

// Add a message to a chat room.
func (r *room) AddMessage(userID UUID, text string, msgType string) {
	// construct message and store
	msg := Message{Text: text, Type: msgType, DateTime: GetTimestamp(), TargetUUID: userID}
	r.messages = append(r.messages, msg)
}

// Send message to all clients in room.
func (r *room) Broadcast(msg Message) {
	for id := range r.userIDs {
		msg.marshalRequestToChan(users[UUID(id)].Out)
	}
}

// Get a formatted date & time stamp.
func GetTimestamp() string {
	return time.Now().Format("_2/01/06 15:04")
}

// Check if a room exists.
func RoomExists(name string) bool {
	_, ok := rooms[name]
	return ok
}

// Represents a single user.
type user struct {
	Name   string
	Online bool
	Out    chan string
}

// Add a new user.
func NewUser(uuid UUID, name string, out chan string) {
	users[uuid] = &user{Name: name, Out: out}
}

// Check if user exists.
func UserExists(name UUID) bool {
	_, ok := users[name]
	return ok
}

// Channel for all std print output.
var stdout = make(chan string)

// Write responses to Stdout.
func writeToStdout() {
	for msg := range stdout {
		fmt.Print(msg)
	}
}

// Format & print input requirement and get console input.
func getConsoleInput(inputMsg string) string {
	reader := bufio.NewReader(os.Stdin)
	stdout <- "> " + inputMsg + ":\n"
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	return text
}

// Format & print input requirement and get console input.
func getConsoleInputRaw() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	return text
}
