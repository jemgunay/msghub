package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
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

func NewTCPServer(host string, port int) error {
	s := &TCPServer{host, port, make(chan struct{}, 1)}
	go requestPoller()
	return s.Start()
}

// Start listening over TCP on a specified port via a specified protocol.
func (s *TCPServer) Start() error {
	// start listener
	listener, err := net.Listen("tcp", s.host+":"+strconv.Itoa(s.port))
	if err != nil {
		return fmt.Errorf("cannot create a TCP listener on %s:%d", s.host, s.port)
	}
	log.Printf("starting TCP server on port %d", s.port)

	// create example rooms
	NewRoom("foo_room", UUID("admin"))
	NewRoom("bar_room", UUID("admin"))

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
	fmt.Println(clientAddress + " TCP client connection accepted")

	// scan input from connection
	input := bufio.NewScanner(conn)
	for input.Scan() {
		// unmarshal client string request into Message object
		msg := Message{}
		msg.unmarshalRequest(input.Text())

		// produce response based on request
		requestPool <- MessageRequest{&msg, ch}
	}

	// client disconnecting
	fmt.Println(clientAddress + " TCP client connection dropped")
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
		users[staleMsg.TargetUUID].out = req.out
		freshMsg.Username = users[UUID(staleMsg.TargetUUID)].name

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

	// list all chat rooms
	case "list":
		staleMsg.Text = ""
		i := 0
		for k := range rooms {
			freshMsg.Text += k
			if i < len(rooms)-1 {
				freshMsg.Text += ", "
			}
			i++
		}

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
		freshMsg.Text = fmt.Sprintf("user '%s' added to the '%s' room", users[UUID(staleMsg.TargetUUID)].name, staleMsg.Room)
		rooms[staleMsg.Room].messages = append(rooms[staleMsg.Room].messages, freshMsg)

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
		freshMsg.Text = fmt.Sprintf("user '%s' removed from the '%s' room", users[UUID(staleMsg.TargetUUID)].name, staleMsg.Room)
		rooms[staleMsg.Room].messages = append(rooms[staleMsg.Room].messages, freshMsg)

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

		// sent new message to all clients in room
		for id := range rooms[staleMsg.Room].userIDs {
			freshMsg.marshalRequestToChan(users[UUID(id)].out)
		}

	default:
		freshMsg.Error = "request type not recognised"
	}

	// marshal request and push to connection writer channel
	freshMsg.marshalRequestToChan(req.out)
}

// Push new TCP messages from channel to connection.
func (s *TCPServer) clientWriter(conn net.Conn, ch <-chan string) {
	for msg := range ch {
		_, err := fmt.Fprintln(conn, msg)
		if err != nil {
			log.Println("Error responding to client: " + err.Error())
		}
	}
}
