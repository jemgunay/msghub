package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"strconv"

	"os"

	"github.com/gorilla/mux"
)

type HTTPServer Server

// Start listening for HTTP requests.
func (s *HTTPServer) Start(host string, port int) error {
	workingDir, err := os.Getwd()

	// define HTTP routes
	router := mux.NewRouter()
	// serve static files
	router.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir(workingDir+"/src/github.com/jemgunay/msghub/static/")))).Methods("GET")
	// chat server related requests
	router.HandleFunc("/", s.handleHTTPRequests).Methods("POST")

	// listen for HTTP requests
	log.Printf("starting HTTP server on port %d", port)
	err = http.ListenAndServe(host+":"+strconv.Itoa(port), router)
	if err != nil {
		return err
	}

	return nil
}

// Process HTTP client requests and handle HTTP responses.
func (s *HTTPServer) handleHTTPRequests(w http.ResponseWriter, req *http.Request) {
	// outgoing client messages
	ch := make(chan string)

	// get URL path components
	urlParts := strings.Split(req.URL.Path, "/")

	// route URL requests
	switch {

	case req.URL.Path == "/":

	// process specific request
	case len(urlParts) > 1:
		// group request information
		w.WriteHeader(http.StatusOK)
		//accessRequest := AccessRequest{store: s.store, methodVerb: req.Method, key: mux.Vars(req)["key"], value: mux.Vars(req)["val"], out: ch}
		//requestPool <- accessRequest

		// send response to client
		s.clientWriter(w, ch)

	// unsupported request
	default:
		// http 404 response
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "page '%s' not found\n", req.URL.Path)
	}
}

// Push new HTTP responses to connection.
func (s *HTTPServer) clientWriter(w http.ResponseWriter, ch <-chan string) {
	for msg := range ch {
		_, err := fmt.Fprintf(w, "%s\n", msg)
		if err != nil {
			log.Println(err)
		}
		return
	}
}
