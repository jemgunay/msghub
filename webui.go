package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type HTTPServer struct {
	Server
	refreshFeed chan string
	client      *Client
}

// Start listening for HTTP requests.
func (s *HTTPServer) Start(client *Client) {
	s.client = client
	s.refreshFeed = make(chan string)

	workingDir, err := os.Getwd()
	if err != nil {
		stdout <- err.Error()
		return
	}

	// define HTTP routes
	router := mux.NewRouter()
	// pull new content from channel
	router.HandleFunc("/refresh/", s.handleRefresh).Methods("GET")
	// retrieve client username
	router.HandleFunc("/fetch/{type}/", s.handleSpecificRequest).Methods("GET")
	// chat server related requests
	router.HandleFunc("/request/", s.handleGeneralRequest).Methods("POST")
	// serve static files
	router.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir(workingDir+"/static/")))).Methods("GET")

	// generate random port for http server and open in browser
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	s.port = client.port + 1 + r1.Intn(1000)
	go openBrowser("http://" + client.host + ":" + strconv.Itoa(s.port))

	// listen for HTTP requests
	log.Printf("starting HTTP server on port %d", s.port)
	err = http.ListenAndServe(client.host+":"+strconv.Itoa(s.port), router)
	if err != nil {
		stdout <- err.Error()
	}
}

// Process HTTP client request to pull fresh chat data.
func (s *HTTPServer) handleRefresh(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)

	select {
	case msg := <-s.refreshFeed:
		_, err := fmt.Fprintf(w, "%s\n", msg)
		if err != nil {
			log.Println(err)
		}
	default:
		_, err := fmt.Fprintf(w, "%s\n", "")
		if err != nil {
			log.Println(err)
		}
	}
}

// Process HTTP client request to set client chat states.
func (s *HTTPServer) handleGeneralRequest(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)

	// perform server request
	req.ParseForm()
	msg := Message{TargetUUID: s.client.clientUUID, Type: req.Form.Get("Type"), Room: req.Form.Get("Room"), Text: req.Form.Get("Text"), DateTime: GetTimestamp()}
	s.client.writeToConnection(s.client.conn, msg)

	// empty http response
	_, err := fmt.Fprintf(w, "%s\n", "")
	if err != nil {
		log.Println(err)
	}
}

// Process HTTP client request for the client username.
func (s *HTTPServer) handleSpecificRequest(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	var err error
	switch mux.Vars(req)["type"] {
	case "name":
		_, err = fmt.Fprintf(w, "%s", s.client.username)
	case "exit":
		s.client.exit <- struct{}{}
		os.Exit(1)
	}

	if err != nil {
		log.Println(err)
	}
}

// Opens the specified URL in the default browser.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
