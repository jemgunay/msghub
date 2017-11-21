package main

import (
	"fmt"
)

// Program entry point.
func main() {
	// continuously write to console output
	go writeToStdout()

	var err error

	// determine if instance is a server or client
	for {
		response := getConsoleInput("client, server or exit")
		switch response {
		case "client":
			err = NewClient("localhost", 8000)
		case "server":
			err = NewServer("localhost", 8000)
		case "exit":
			return
		default:
			fmt.Printf("Input must be 'client', 'server' or 'exit'.\n")
		}

		// report service start failure
		if err != nil {
			fmt.Println(err.Error())
		}
	}
}
