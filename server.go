package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
)

func main() {
	InitLogger()

	connChan := make(chan net.Conn, 100)
	gameRequestCh := make(chan GameRequest, 100)

	go ConnectionHandler(connChan, gameRequestCh)
	go GameHandler(gameRequestCh)

	server := &Server{
		Type: "tcp",
		Host: "",
		Port: 6000,
	}
	server.Run(connChan)
}

type Server struct {
	Type string
	Host string
	Port int
}

func (s *Server) Run(connChan chan net.Conn) {
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

func GetPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "6000"
		log.Printf("No PORT environment variable, setting to %s", port)
	}
	return ":" + port
}

func InitLogger() {
	f, err := os.OpenFile("sag.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
}
