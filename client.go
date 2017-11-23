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

	"strings"

	"github.com/twinj/uuid"
)

// A chat client instance.
type Client struct {
	host       string
	port       int
	exit       chan struct{}
	clientUUID UUID
	username   string
	httpServer HTTPServer
	protocol   string
	conn       net.Conn
}

var uuidFilePath string

func NewClient(host string, port int) error {
	// continuously process stdin console input
	input := getConsoleInput("Protocol: tcp or udp (default tcp)")
	if input != "tcp" && input != "udp" {
		input = "tcp"
	}

	c := &Client{host: host, port: port, exit: make(chan struct{}, 1), protocol: input}
	return c.Start()
}

// Start a new client instance.
func (c *Client) Start() error {
	// connect to server
	conn, err := net.Dial(c.protocol, c.host+":"+strconv.Itoa(c.port))
	if err != nil {
		return err
	}
	defer conn.Close()
	c.conn = conn

	// assign UUID for this client
	c.clientUUID = c.initUUID(conn)

	// start HTTP server to access web UI
	go c.httpServer.Start(c)

	// continuously read from connection
	go c.readFromConnection(conn)

	// continuously process stdin console input
	go func() {
		for {
			input := getConsoleInputRaw()

			switch input {
			// request chat room list
			case "list":
				newMsg := Message{Type: "list", TargetUUID: c.clientUUID, DateTime: GetTimestamp()}
				c.writeToConnection(conn, newMsg)

			// exit client
			case "exit":
				c.exit <- struct{}{}
				os.Exit(1)

			// multi-parameter commands
			default:
				// ensure parameter length hof 2
				inputComponents := strings.Split(input, " ")
				if len(inputComponents) < 2 {
					stdout <- "Unsupported command: too few parameters.\n"
					continue
				}

				var newMsg Message
				switch inputComponents[0] {
				// create chat room
				case "create":
					newMsg = Message{Type: "create", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: inputComponents[1]}

				// destroy chat room
				case "destroy":
					newMsg = Message{Type: "destroy", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: inputComponents[1]}

				// join chat room
				case "join":
					newMsg = Message{Type: "join", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: inputComponents[1]}

				// leave chat room
				case "leave":
					newMsg = Message{Type: "leave", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: inputComponents[1]}

				// send msg to chat room
				default:
					msgText := strings.Join(inputComponents[1:], ",")
					newMsg = Message{Type: "new_msg", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: inputComponents[0], Text: msgText}
				}

				c.writeToConnection(conn, newMsg)
			}
		}
	}()

	// request chat room list
	newMsg := Message{Type: "list", TargetUUID: c.clientUUID, DateTime: GetTimestamp()}
	c.writeToConnection(conn, newMsg)

	// join chat room
	//newMsg = Message{Type: "join", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: "room_1"}
	//c.writeToConnection(conn, newMsg)

	<-c.exit
	return nil
}

// Read messages from connection.
func (c *Client) readFromConnection(conn net.Conn) {
	// continuously poll for messages
	for {
		response, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Fatalln("> Server closed connection.")
		}

		// pass new response to web UI feed
		go func() {
			c.httpServer.refreshFeed <- response
		}()

		// push request job into channel for processing
		var msg Message
		msg.unmarshalRequest(response)
		c.processResponse(msg)
	}
}

// Direct server responses to corresponding methods.
func (c *Client) processResponse(msg Message) {
	// check for errors returned by server
	if msg.Error != "" {
		stdout <- "> Request error: " + msg.Error + "\n"
		return
	}

	switch msg.Type {
	// join server for the first time
	case "set_name":
		stdout <- msg.Text + "\n"

	// create a chat room
	case "create":
		stdout <- msg.Text + "\n"

	// destroy a chat room
	case "destroy":
		if msg.Username == c.username {
			stdout <- fmt.Sprintf("[%s]: You have destroyed the room.\n", msg.Room)
			return
		}
		stdout <- fmt.Sprintf("[%s]: This room has been destroyed by '%s'.\n", msg.Room, msg.Username)

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
	_, err = fmt.Fprintf(conn, str+"\n")
	if err != nil {
		log.Printf(err.Error())
	}
}

// Read UUID from file or generate a new one if file does not exist.
func (c *Client) initUUID(conn net.Conn) UUID {
	// UUID file path
	workingDir, err := os.Getwd()
	name := getConsoleInput("Enter new or previously used user name")
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
