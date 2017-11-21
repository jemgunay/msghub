// A basic console client for communicating to UDP & TCP servers.
package main

import (
	"log"
	"net"
	"os"

	"io/ioutil"

	"bufio"
	"fmt"

	"strconv"

	"github.com/twinj/uuid"
)

// A chat client instance.
type Client struct {
	host       string
	port       int
	exit       chan struct{}
	clientUUID UUID
	username   string
}

var uuidFilePath string

func NewClient(host string, port int) error {
	c := &Client{host: host, port: port, exit: make(chan struct{}, 1)}
	return c.Start()
}

// Start a new client instance.
func (c *Client) Start() error {
	// connect to server
	conn, err := net.Dial("tcp", c.host+":"+strconv.Itoa(c.port))
	if err != nil {
		return err
	}
	defer conn.Close()

	// assign UUID for this client
	c.clientUUID = c.initUUID(conn)

	// continuously read from connection
	go c.readFromConnection(conn)

	// continuously process stdin console input
	go func() {
		for {
			input := getConsoleInputRaw()
			// exit
			switch input {
			case "exit":
				c.exit <- struct{}{}

			case "leave":
				// leave chat room
				newMsg := Message{Type: "leave", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: "foo_room"}
				c.writeToConnection(conn, newMsg)

			case "join":
				// join chat room
				newMsg := Message{Type: "join", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: "foo_room"}
				c.writeToConnection(conn, newMsg)

			default:
				// send msg to chat room
				newMsg := Message{Type: "new_msg", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: "foo_room", Text: input}
				c.writeToConnection(conn, newMsg)
			}
		}
	}()

	// request chat room list
	newMsg := Message{Type: "list", TargetUUID: c.clientUUID, DateTime: GetTimestamp()}
	c.writeToConnection(conn, newMsg)

	// join chat room
	newMsg = Message{Type: "join", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: "foo_room"}
	c.writeToConnection(conn, newMsg)

	<-c.exit
	return nil
}

// Read messages from connection.
func (c *Client) readFromConnection(conn net.Conn) {
	// continuously poll for messages
	for {
		request, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Fatalln("> Server closed connection.")
		}

		// push request job into channel for processing
		var msg Message
		msg.unmarshalRequest(request)
		c.processResponse(msg)
	}
}

// Direct server responses to corresponding methods.
func (c *Client) processResponse(msg Message) {
	// check for errors returned by server
	if msg.Error != "" {
		stdout <- "> Request error: " + msg.Error + "\n"
	}

	switch msg.Type {
	// join server for the first time
	case "set_name":
		stdout <- msg.Text + "\n"

	// join server for the first time
	case "list":
		if len(msg.Text) == 0 {
			stdout <- "No rooms available\n"
		}
		stdout <- "Available chat rooms: " + msg.Text + "\n"

	// join a chat room
	case "join":
		if msg.Username == c.username {
			stdout <- fmt.Sprintf("[%s] %s: %s\n", msg.Room, msg.Username, "You have joined the room.")
			return
		}
		stdout <- fmt.Sprintf("[%s] %s: %s\n", msg.Room, msg.Username, "Joined the room.")

	// leave chat room
	case "leave":
		if msg.Username == c.username {
			stdout <- fmt.Sprintf("[%s] %s: %s\n", msg.Room, msg.Username, "You are leaving the room.")
			return
		}
		stdout <- fmt.Sprintf("[%s] %s: %s\n", msg.Room, msg.Username, "Left the room.")

	// a standard message to a server room
	case "new_msg":
		stdout <- fmt.Sprintf("[%s] %s: %s\n", msg.Room, msg.Username, msg.Text)

	default:
		stdout <- "> Request message type not recognised\n"
	}
}

// Write message to connection.
func (c *Client) writeToConnection(conn net.Conn, msg Message) {
	str, err := msg.marshalRequest()
	if err != nil {
		log.Printf(err.Error())
	}
	fmt.Fprintf(conn, str+"\n")
}

// Read UUID from file or generate a new one if file does not exist.
func (c *Client) initUUID(conn net.Conn) UUID {
	// UUID file path
	workingDir, err := os.Getwd()
	name := getConsoleInput("Enter new or previously used user name")
	//uuidFilePath = workingDir + "/src/github.com/jemgunay/msghub/client.dat"
	uuidFilePath = workingDir + "/data/" + name + ".dat"

	// attempt to read UUID from file
	uuid, err := c.readUUIDFromFile(uuidFilePath)
	if err != nil {
		// file did not exist, generate new UUID and save to new file
		uuid, err = c.generateUUIDFile(workingDir + "/data/" + name + ".dat")
		if err != nil {
			log.Fatal("Could not locate existing or generate new client ID.")
		}

		// set new user name on server
		nameMsg := Message{Type: "set_name", TargetUUID: UUID(uuid), DateTime: GetTimestamp(), Text: name}
		c.writeToConnection(conn, nameMsg)
	}

	c.username = name
	return UUID(uuid)
}

// Read UUID from file.
func (c *Client) readUUIDFromFile(filePath string) (UUID string, err error) {
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(contents), nil
}

// Generate new UUID and save it to new file.
func (c *Client) generateUUIDFile(filePath string) (UUID string, err error) {
	uuid := uuid.NewV4().String()
	// create file to save UUID to
	file, err := os.Create(filePath)
	file.Close()
	if err != nil {
		return uuid, err
	}
	// write UUID to file
	err = ioutil.WriteFile(filePath, []byte(uuid), 0644)

	return uuid, err
}
