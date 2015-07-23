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
	ch       chan Message
}

type Message struct {
	from Client
	text string
}

type Room struct {
	name    string
	clients map[net.Conn]*Client
	addchan chan *Client
	rmchan  chan Client
	msgchan chan Message
}

func main() {
	ln, err := net.Listen("tcp", ":6000")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rooms := make(map[string]*Room)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go handleConnection(conn, rooms)
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
			from: c,
			text: line,
		}
		ch <- message
	}
}

func (c Client) WriteLinesFrom(ch <-chan Message) {
	for msg := range ch {
		if msg.from != c {
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
	io.WriteString(c, "Log in to your team's assigned collaboration channel")
	room, _, _ := bufc.ReadLine()
	return string(room)
}

func handleConnection(c net.Conn, rooms map[string]*Room) {
	bufc := bufio.NewReader(c)
	defer c.Close()
	client := Client{
		conn:     c,
		nickname: promptNick(c, bufc),
		ch:       make(chan Message),
	}
	if strings.TrimSpace(client.nickname) == "" {
		io.WriteString(c, "Invalid Username\n")
		return
	}
	welcomeMsg := `A monolithic building appears before you. You have arrived
				   at the office. Try not to act suspicious.`
	io.WriteString(c, welcomeMsg)

	roomName := promptRoom(c, bufc)
	room, ok := rooms[roomName]
	if !ok {
		// Create a new room with name
		room = &Room{
			name:    roomName,
			clients: make(map[net.Conn]*Client, 0),
			addchan: make(chan *Client),
			rmchan:  make(chan Client),
			msgchan: make(chan Message),
		}
		rooms[roomName] = room
		go handleIO(rooms[roomName])
	}

	if len(room.clients) < 3 {
		// Register user
		room.addchan <- &client
		defer func() {
			room.msgchan <- Message{
				text: fmt.Sprintf("%s has left %s.\n", client.nickname, room.name),
			}
			log.Printf("Connection from %v closed.\n", c.RemoteAddr())
			room.rmchan <- client
		}()
		room.msgchan <- Message{
			text: fmt.Sprintf("--> | %s has joined %s, waiting for teammates...\n", client.nickname, room.name),
		}

	} else {
		fullMsg := `I'm sorry, it seems your teammates have started without
		            you. Please try again`
		io.WriteString(c, fullMsg)
	}
	// if everyone is present, start accepting I/O
	if len(room.clients) == 3 {
		startMsg := `*-- | Everyone has arrived, mission starting...
		             Ask for /help to get familiar around here`
		io.WriteString(c, startMsg)
		go client.WriteLinesFrom(client.ch)
		client.ReadLinesInto(room.msgchan)
	}
}

func handleIO(r *Room) {
	// handle all io from clients
	for {
		select {
		case msg := <-r.msgchan:
			log.Printf("%s: %s", msg.from.nickname, msg.text)
			for _, client := range r.clients {
				client.ch <- msg
			}
		case client := <-r.addchan:
			log.Printf("New client: %v\n", client.conn)
			r.clients[client.conn] = client
		case client := <-r.rmchan:
			log.Printf("Client disconnects: %v\n", client.conn)
			delete(r.clients, client.conn)
		default:
		}
	}
}

func printHelp(ch chan<- Message) {
	msg := `help -- |  Usage:
            help -- |
            help -- |     /[cmd] [arguments]
            help -- |
            help -- |  Available commands:
            help -- |
            help -- |    /msg [to] [text]         send message to coworker
            help -- |    /list                    look at files you have access to
            help -- |    /send [to] [filename]    move file to coworker
            help -- |    /look                    show coworkers`
	ch <- Message{text: msg}
}
