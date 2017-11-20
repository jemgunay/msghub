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
	stdout     chan string
	clientUUID UUID
}

var uuidFilePath string

func NewClient(host string, port int) error {
	c := &Client{host, port, make(chan struct{}, 1), make(chan string), ""}
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

	// continuously write to console output
	go c.writeToStdout()
	// continuously read from connection
	go c.readFromConnection(conn)

	// request chat room list
	newMsg := Message{Type: "list", TargetUUID: c.clientUUID, DateTime: GetTimestamp()}
	c.writeToConnection(conn, newMsg)

	// join chat room
	newMsg = Message{Type: "join", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: "foo_room"}
	c.writeToConnection(conn, newMsg)

	// send msg to chat room
	newMsg = Message{Type: "new_msg", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: "foo_room", Text: "this is a test message"}
	c.writeToConnection(conn, newMsg)

	// leave chat room
	newMsg = Message{Type: "leave", TargetUUID: c.clientUUID, DateTime: GetTimestamp(), Room: "foo_room"}
	c.writeToConnection(conn, newMsg)

	// continuously process console input
	go func() {
		for {
			input := getConsoleInput("type 'exit' to exit")
			// exit
			if input == "exit" {
				c.exit <- struct{}{}
			}
		}
	}()

	<-c.exit
	return nil
}

// Read messages from connection.
func (c *Client) readFromConnection(conn net.Conn) {
	// continuously poll for messages
	for {
		request, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Fatal(err.Error())
		}

		// push request job into channel for processing
		var msg Message
		msg.unmarshalRequest(request)
		c.processResponse(msg)
	}
}

// Direct server responses to corresponding methods.
func (c *Client) processResponse(msg Message) error {
	// check for errors returned by server
	if msg.Error != "" {
		c.stdout <- msg.Error
		return fmt.Errorf(msg.Error)
	}

	switch msg.Type {
	// join server for the first time
	case "set_name":
		//fmt.Println(msg.Text)
		c.stdout <- msg.Text

	// join server for the first time
	case "list":
		if len(msg.Text) == 0 {
			//fmt.Println("no rooms available")
			c.stdout <- "no rooms available"
		}
		//fmt.Println("chat rooms: " + msg.Text)
		c.stdout <- "chat rooms: " + msg.Text

	// join a chat room
	case "join":
		//fmt.Println(msg.Text)
		c.stdout <- msg.Text

	// leave chat room
	case "leave":
		//fmt.Println(msg.Text)
		c.stdout <- msg.Text
		c.stdout <- fmt.Sprintf("[%s] %s: %s", msg.Room, msg.Username, msg.Text)

	// a standard message to a server room
	case "new_msg":
		c.stdout <- fmt.Sprintf("[%s] %s: %s", msg.Room, msg.Username, msg.Text)

	default:
		return fmt.Errorf("request message type not recognised")
	}

	return nil
}

// Write message to connection.
func (c *Client) writeToConnection(conn net.Conn, msg Message) {
	str, err := msg.marshalRequest()
	if err != nil {
		log.Printf(err.Error())
	}
	fmt.Fprintf(conn, str+"\n")
}

// Write responses to Stdout.
func (c *Client) writeToStdout() {
	for msg := range c.stdout {
		fmt.Println(msg)
	}
}

// Read UUID from file or generate a new one if file does not exist.
func (c *Client) initUUID(conn net.Conn) UUID {
	// UUID file path
	clientFile := getConsoleInput("enter .dat user file (leave blank to create new user)")
	workingDir, err := os.Getwd()
	//uuidFilePath = workingDir + "/src/github.com/jemgunay/msghub/client.dat"
	uuidFilePath = workingDir + "/" + clientFile

	// attempt to read UUID from file
	uuid, err := c.readUUIDFromFile(uuidFilePath)
	if err != nil {
		// get new user name
		name := getConsoleInput("enter new username")

		// file did not exist, generate new UUID and save to new file
		uuid, err = c.generateUUIDFile(workingDir + "/" + name + ".dat")
		if err != nil {
			log.Fatal("Could not locate existing or generate new client ID.")
		}

		// set new user name on server
		nameMsg := Message{Type: "set_name", TargetUUID: UUID(uuid), DateTime: GetTimestamp(), Text: name}
		c.writeToConnection(conn, nameMsg)
	}

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
