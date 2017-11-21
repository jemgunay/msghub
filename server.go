package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
)

// Supported service Protocol types.
type Protocol string

const (
	TCP  Protocol = "tcp"
	UDP  Protocol = "udp"
	HTTP Protocol = "http"
)

// Server types.
type Server struct {
	host string
	port int
	exit chan struct{}
}
type TCPServer Server
type UDPServer Server

func NewServer(host string, port int) error {
	s := &TCPServer{host, port, make(chan struct{}, 1)}
	go requestPoller()
	return s.Start()
}

// Start listening over TCP on a specified port via a specified protocol.
func (s *TCPServer) Start() error {
	err := unpackChatServer()
	if err != nil {
		log.Println(err.Error())
	}

	// start listener
	listener, err := net.Listen("tcp", s.host+":"+strconv.Itoa(s.port))
	if err != nil {
		return fmt.Errorf("cannot create a TCP listener on %s:%d", s.host, s.port)
	}
	log.Printf("starting TCP server on port %d", s.port)

	// create example rooms
	NewRoom("room_1", UUID("admin"))
	NewRoom("room_2", UUID("admin"))

	// listen for new connections
	for {
		conn, err := listener.Accept()
		// connection error
		if err != nil {
			log.Print(err)
			continue
		}

		// handle connection
		go s.handleConn(conn)
	}

	<-s.exit
	return nil
}

// Process newly accepted TCP connection and associated client.
func (s *TCPServer) handleConn(conn net.Conn) {
	defer conn.Close()

	// outgoing client messages
	ch := make(chan string)
	defer close(ch)

	// send any new msg through connection
	go s.clientWriter(conn, ch)

	// get client address
	clientAddress := conn.RemoteAddr().String()
	fmt.Println(clientAddress + " TCP client connection established")

	// scan input from connection
	var clientUUID UUID
	input := bufio.NewScanner(conn)
	for input.Scan() {
		// unmarshal client string request into Message object
		msg := Message{}
		msg.unmarshalRequest(input.Text())
		clientUUID = msg.TargetUUID

		// produce response based on request
		requestPool <- MessageRequest{&msg, ch}
	}

	// client disconnecting
	fmt.Println(clientAddress + " TCP client connection dropped")
	// broadcast user leaving message to all room users
	exitMsg := Message{TargetUUID: clientUUID, Type: "exit"}
	requestPool <- MessageRequest{msg: &exitMsg}
}

// A message request format accepted by the request poller.
type MessageRequest struct {
	msg *Message
	out chan string
}

var requestPool chan MessageRequest

// Poll for requests to process.
func requestPoller() {
	requestPool = make(chan MessageRequest)

	for currentRequest := range requestPool {
		currentRequest.processRequest()
	}
}

// Direct requests to corresponding methods.
func (req *MessageRequest) processRequest() {
	staleMsg := req.msg
	freshMsg := Message{Type: staleMsg.Type, Room: staleMsg.Room, DateTime: GetTimestamp()}

	// validate
	if UserExists(staleMsg.TargetUUID) {
		// update use references to output channel and msg username
		users[staleMsg.TargetUUID].Out = req.out
		freshMsg.Username = users[UUID(staleMsg.TargetUUID)].Name

	} else if staleMsg.Type != "set_name" {
		// if user does not exist and request is not a 'create' request, then exit
		freshMsg.Error = "no name is associated with client ID - set a user name first"
		freshMsg.marshalRequestToChan(req.out)
		return
	}

	switch staleMsg.Type {

	// join server for the first time
	case "set_name":
		NewUser(staleMsg.TargetUUID, staleMsg.Text, req.out)
		log.Printf("user with UUID '%s' set their name to '%s'", staleMsg.TargetUUID, staleMsg.Text)
		freshMsg.Text = fmt.Sprintf("user name successfully set to '%s'", staleMsg.Text)

		freshMsg.marshalRequestToChan(req.out)

	// list all chat rooms
	case "list":
		i := 0
		for k := range rooms {
			freshMsg.Text += k
			if i < len(rooms)-1 {
				freshMsg.Text += ", "
			}
			i++
		}

		freshMsg.marshalRequestToChan(req.out)

	// join a chat room
	case "join":
		if RoomExists(staleMsg.Room) == false {
			freshMsg.Error = "specified room does not exist"
			break
		}
		// check if user is subscribed to the room
		if rooms[staleMsg.Room].IsUserSubscribed(staleMsg.TargetUUID) {
			freshMsg.Error = "user is already subscribed to this room"
			break
		}
		rooms[staleMsg.Room].AddUser(staleMsg.TargetUUID)
		freshMsg.Text = fmt.Sprintf("user '%s' added to the '%s' room", users[UUID(staleMsg.TargetUUID)].Name, staleMsg.Room)
		rooms[staleMsg.Room].messages = append(rooms[staleMsg.Room].messages, freshMsg)

		rooms[staleMsg.Room].Broadcast(freshMsg)

	// leave chat room
	case "leave":
		if RoomExists(staleMsg.Room) == false {
			freshMsg.Error = "specified room does not exist"
			break
		}
		// check if user is subscribed to the room
		if rooms[staleMsg.Room].IsUserSubscribed(staleMsg.TargetUUID) == false {
			freshMsg.Error = "user is not subscribed to this room."
			break
		}
		rooms[staleMsg.Room].RemoveUser(staleMsg.TargetUUID)
		freshMsg.Text = fmt.Sprintf("user '%s' removed from the '%s' room", users[UUID(staleMsg.TargetUUID)].Name, staleMsg.Room)
		rooms[staleMsg.Room].messages = append(rooms[staleMsg.Room].messages, freshMsg)

		rooms[staleMsg.Room].Broadcast(freshMsg)

	// a standard message to server
	case "new_msg":
		if RoomExists(staleMsg.Room) == false {
			freshMsg.Error = "specified room does not exist"
			break
		}
		// check if user is subscribed to the room
		if rooms[staleMsg.Room].IsUserSubscribed(staleMsg.TargetUUID) == false {
			freshMsg.Error = "user is not subscribed to this room."
			break
		}
		// add msg to room records
		rooms[staleMsg.Room].messages = append(rooms[staleMsg.Room].messages, freshMsg)
		freshMsg.Text = staleMsg.Text

		// broadcast to all clients subscribed to room
		rooms[staleMsg.Room].Broadcast(freshMsg)

	// client connection dropped
	case "exit":
		// unsubscribe user from each room
		for name := range rooms {
			// check if user is subscribed to the room
			if rooms[name].IsUserSubscribed(staleMsg.TargetUUID) == false {
				continue
			}

			// broadcast user leaving message to everyone in room
			rooms[name].RemoveUser(staleMsg.TargetUUID)
			freshMsg.Text = fmt.Sprintf("user '%s' removed from the '%s' room", users[UUID(staleMsg.TargetUUID)].Name, name)
			freshMsg.Type = "leave"
			freshMsg.Room = name
			rooms[name].messages = append(rooms[name].messages, freshMsg)

			rooms[name].Broadcast(freshMsg)
		}

	default:
		freshMsg.Error = "request type not recognised"
	}

	// update user persistence file
	err := storeChatServer()
	if err != nil {
		log.Println(err.Error())
	}
}

// Pull new TCP messages from channel to connection.
func (s *TCPServer) clientWriter(conn net.Conn, ch <-chan string) {
	for msg := range ch {
		_, err := fmt.Fprintln(conn, msg)
		if err != nil {
			log.Println("Error responding to client: " + err.Error())
		}
	}
}

// Store server user and room data to file.
func storeChatServer() error {
	workingDir, err := os.Getwd()

	// create/truncate file for writing to
	file, err := os.Create(workingDir + "/src/github.com/jemgunay/msghub/data/users.dat")
	defer file.Close()
	if err != nil {
		return err
	}

	// encode store map to file
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(&users)
	if err != nil {
		return err
	}

	return nil
}

// Unpack server user and room data from file.
func unpackChatServer() error {
	workingDir, err := os.Getwd()

	// open file to read from
	file, err := os.Open(workingDir + "/src/github.com/jemgunay/msghub/data/users.dat")
	defer file.Close()
	if err != nil {
		return err
	}

	// decode file contents to store map
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&users)
	if err != nil {
		return err
	}

	return nil
}
