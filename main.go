package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

type Client struct {
	conn     net.Conn
	nickname string
	room     string
	ch       chan Message
}

type Message struct {
	client Client
	text   string
}

func main() {
	ln, err := net.Listen("tcp", ":6000")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	msgchan := make(chan Message)
	addchan := make(chan Client)
	rmchan := make(chan Client)

	go handleMessages(msgchan, addchan, rmchan)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go handleConnection(conn, msgchan, addchan, rmchan)
	}
}

func (c Client) ReadLinesInto(ch chan<- Message) {
	bufc := bufio.NewReader(c.conn)
	for {
		line, err := bufc.ReadString('\n')
		if err != nil {
			break
		}
		message := Message{
			client: c,
			text:   fmt.Sprintf("%s: %s", c.nickname, line),
		}
		ch <- message
	}
}

func (c Client) WriteLinesFrom(ch <-chan Message) {
	for msg := range ch {
		if msg.client.room == c.room && msg.client != c {
			_, err := io.WriteString(c.conn, msg.text)
			if err != nil {
				return
			}
		}
	}
}

func promptNick(c net.Conn, bufc *bufio.Reader) string {
	io.WriteString(c, "Enter a nickname: ")
	nick, _, _ := bufc.ReadLine()
	return string(nick)
}

func promptRoom(c net.Conn, bufc *bufio.Reader) string {
	io.WriteString(c, "Enter a room: ")
	room, _, _ := bufc.ReadLine()
	return string(room)
}

func handleConnection(c net.Conn, msgchan chan<- Message, addchan chan<- Client, rmchan chan<- Client) {
	bufc := bufio.NewReader(c)
	defer c.Close()
	client := Client{
		conn:     c,
		nickname: promptNick(c, bufc),
		room:     promptRoom(c, bufc),
		ch:       make(chan Message),
	}
	if strings.TrimSpace(client.nickname) == "" {
		io.WriteString(c, "Invalid Username\n")
		return
	}

	// Register user
	addchan <- client
	defer func() {
		msgchan <- Message{
			text: fmt.Sprintf("User %s left the chat room.\n", client.nickname),
		}
		log.Printf("Connection from %v closed.\n", c.RemoteAddr())
		rmchan <- client
	}()
	io.WriteString(c, fmt.Sprintf("Welcome to room %s, %s!\n\n", client.room, client.nickname))
	msgchan <- Message{
		text: fmt.Sprintf("New user %s has joined the chat room.\n", client.nickname),
	}

	// I/O
	go client.WriteLinesFrom(client.ch)
	client.ReadLinesInto(msgchan)
}

func handleMessages(msgchan <-chan Message, addchan <-chan Client, rmchan <-chan Client) {
	clients := make(map[net.Conn]chan<- Message)

	for {
		select {
		case msg := <-msgchan:
			log.Printf("New message: %s: %s", msg.client.nickname, msg.text)
			for _, ch := range clients {
				go func(mch chan<- Message) { mch <- msg }(ch)
			}
		case client := <-addchan:
			log.Printf("New client: %v\n", client.conn)
			clients[client.conn] = client.ch
		case client := <-rmchan:
			log.Printf("Client disconnects: %v\n", client.conn)
			delete(clients, client.conn)
		default:
		}
	}
}
