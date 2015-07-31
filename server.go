package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
)

func main() {
	clientChan := make(chan *Client, 100)
	connChan := make(chan net.Conn, 100)
	go ConnectionHandler(connChan, clientChan)
	go GameHandler(clientChan)

	server := &Server{
		Type: "tcp",
		Host: "127.0.0.1",
		Port: 6000,
	}
	server.Start(connChan)
}

type Server struct {
	Type string
	Host string
	Port int
}

func (s *Server) Start(connChan chan net.Conn) {
	address := s.Host + ":" + strconv.Itoa(s.Port)
	ln, err := net.Listen(s.Type, address)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	log.Println("Now accepting connections on port 6000")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Error occurred accepting connection: %p", err.Error())
			continue
		}
		log.Printf("New connection from %v", conn.RemoteAddr())
		connChan <- conn
	}
}

func ConnectionHandler(c <-chan net.Conn, ch chan *Client) {
	for {
		select {
		case conn := <-c:
			go InitClient(conn, ch)
		}
	}
}
