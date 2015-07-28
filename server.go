package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	ln, err := net.Listen("tcp", ":6000")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	log.Println("Now accepting connections on port 6000")

	// game channel that handles requests for new games
	clientChan := make(chan *Client, 5)
	go GameHandler(clientChan)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Error occurred accepting connection: %s", err.Error())
			continue
		}

		go ConnectionHandler(conn, clientChan)
	}
}

func ConnectionHandler(c net.Conn, ch chan *Client) {
	defer c.Close()

	client := NewClient(c)
	ch <- client

	<-client.Done
}
