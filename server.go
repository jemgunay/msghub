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
	// populate server data stores from file
	err := unpackChatServer()
	if err != nil {
		log.Println(err.Error())
	}

	// create example rooms
	NewRoom("room_1", UUID("admin"))
	NewRoom("room_2", UUID("admin"))

	go requestPoller()

	// start TCP & UDP servers
	ts := &TCPServer{host, port, make(chan struct{}, 1)}
	us := &UDPServer{host, port, make(chan struct{}, 1)}

	// return first error to caller
	var errors chan error
	go ts.Start(errors)
	go us.Start(errors)

	// continuously process stdin console input
	func() {
		for {
			input := getConsoleInputRaw()
			switch input {
			// exit server
			case "exit":
				ts.exit <- struct{}{}
				us.exit <- struct{}{}
				errors <- nil
				return
			}
		}
	}()

	return <-errors
}

// Start listening over TCP on a specified port via a specified protocol.
func (s *TCPServer) Start(errors chan error) {
	// start listener
	listener, err := net.Listen("tcp", s.host+":"+strconv.Itoa(s.port))
	if err != nil {
		errors <- fmt.Errorf("cannot create a TCP listener on %s:%d", s.host, s.port)
		return
	}
	log.Printf("starting TCP server on port %d", s.port)

	// listen for new connections
	go func() {
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
	}()

	<-s.exit
	errors <- nil
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

// Pull new TCP messages from channel to connection.
func (s *TCPServer) clientWriter(conn net.Conn, ch <-chan string) {
	for msg := range ch {
		_, err := fmt.Fprintln(conn, msg)
		if err != nil {
			log.Println("Error responding to client: " + err.Error())
		}
	}
}

// Start listening over UDP on a specified port via a specified protocol.
func (s *UDPServer) Start(errors chan error) {
	// prepare UDP server address
	udpAddr := net.UDPAddr{
		Port: s.port,
		IP:   net.ParseIP(s.host),
	}

	// create UDP listener
	listener, err := net.ListenUDP("udp", &udpAddr)
	if err != nil {
		errors <- fmt.Errorf("cannot create a UDP listener on %s:%d", s.host, s.port)
		return
	}
	log.Printf("starting UDP server on port %d", s.port)

	// constantly poll for udp requests
	go func() {
		for {
			// read from UDP connection to buffer
			buffer := make([]byte, 2048)
			n, remoteAddr, err := listener.ReadFromUDP(buffer)
			// read error
			if err != nil {
				log.Print(err)
				continue
			}

			// create string form byte array buffer
			request := string(buffer[:n])

			// handle request
			go s.handleConn(listener, remoteAddr, request)
		}
	}()

	<-s.exit
	errors <- nil
}

// Process newly accepted UDP connection and associated client.
func (s *UDPServer) handleConn(conn *net.UDPConn, addr *net.UDPAddr, request string) {
	// outgoing client messages
	ch := make(chan string)

	// send response to client
	go s.clientWriter(conn, addr, ch)

	// get client address
	fmt.Println(addr.String() + " UDP client request received")

	// unmarshal client string request into Message object
	msg := Message{}
	msg.unmarshalRequest(request)

	// produce response based on request
	requestPool <- MessageRequest{&msg, ch}
}

// Push new UDP messages from channel to connection.
func (s *UDPServer) clientWriter(conn *net.UDPConn, addr *net.UDPAddr, ch <-chan string) {
	for msg := range ch {
		_, err := conn.WriteToUDP([]byte(msg+"\n"), addr)
		if err != nil {
			fmt.Printf("Couldn't send UDP response %v", err)
		}
	}

	// client disconnecting
	fmt.Println(addr.String() + " UDP response transmitted to client")
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

	// update user persistence file
	err := storeChatServer()
	if err != nil {
		log.Println(err.Error())
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
		return

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
		freshMsg.Text = fmt.Sprintf("user '%s' removed from the '%s' room", users[UUID(staleMsg.TargetUUID)].Name, staleMsg.Room)
		rooms[staleMsg.Room].messages = append(rooms[staleMsg.Room].messages, freshMsg)

		rooms[staleMsg.Room].Broadcast(freshMsg)
		rooms[staleMsg.Room].RemoveUser(staleMsg.TargetUUID)
		return

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
		return

	// client connection dropped
	case "exit":
		// unsubscribe user from each room
		for name := range rooms {
			// check if user is subscribed to the current room
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
		return

	default:
		freshMsg.Error = "request type not recognised"
	}

	// if request was not broadcasted above, then send response to the client who made the request only
	freshMsg.marshalRequestToChan(req.out)
}

// Store server user and room data to file.
func storeChatServer() error {
	workingDir, err := os.Getwd()

	// create/truncate file for writing to
	file, err := os.Create(workingDir + "/data/users.dat")
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
	file, err := os.Open(workingDir + "/data/users.dat")
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
