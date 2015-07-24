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
	Conn     net.Conn
	Nickname string
	Ch       chan Message
}

type Message struct {
	From Client
	Text string
}

type Room struct {
	Name    string
	Clients map[net.Conn]*Client
	Addchan chan *Client
	Rmchan  chan Client
	Msgchan chan Message
}

func main() {
	ln, err := net.Listen("tcp", ":6000")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rooms := make(map[string]*Room)

	for {
		Conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go handleConnection(Conn, rooms)
	}
}

func (c Client) ReadLinesInto(ch chan<- Message) {
	bufc := bufio.NewReader(c.Conn)
	for {
		line, err := bufc.ReadString('\n')
		if err != nil {
			break
		}
		message := Message{
			From: c,
			Text: line,
		}
		ch <- message
	}
}

func (c Client) WriteLinesFrom(ch <-chan Message) {
	for msg := range ch {
		if msg.From != c {
			_, err := io.WriteString(c.Conn, msg.Text)
			if err != nil {
				return
			}
		}
	}
}

func promptNick(c net.Conn, bufc *bufio.Reader) string {
	io.WriteString(c, "Enter a Nickname: ")
	nick, _, _ := bufc.ReadLine()
	return string(nick)
}

func promptRoom(c net.Conn, bufc *bufio.Reader) string {
	io.WriteString(c, "Log in to your team's assigned collaboration channel: ")
	room, _, _ := bufc.ReadLine()
	return string(room)
}

func handleConnection(c net.Conn, rooms map[string]*Room) {
	bufc := bufio.NewReader(c)
	defer c.Close()
	client := Client{
		Conn:     c,
		Nickname: promptNick(c, bufc),
		Ch:       make(chan Message),
	}
	if strings.TrimSpace(client.Nickname) == "" {
		io.WriteString(c, "Invalid Username\n")
		return
	}
	welcomeMsg := "A monolithic building appears before you. You have arrived\n" +
		"at the office. Try not to act suspicious.\n\n"
	io.WriteString(c, welcomeMsg)

	roomName := promptRoom(c, bufc)
	room, ok := rooms[roomName]
	if !ok {
		// Create a new room with name
		room = &Room{
			Name:    roomName,
			Clients: make(map[net.Conn]*Client, 0),
			Addchan: make(chan *Client),
			Rmchan:  make(chan Client),
			Msgchan: make(chan Message),
		}
		rooms[roomName] = room
		go handleIO(rooms[roomName])
	}

	if len(room.Clients) < 3 {
		room.Addchan <- &client
		room.Msgchan <- Message{
			Text: fmt.Sprintf("--> | %s has joined %s, waiting for teammates...\n", client.Nickname, room.Name),
		}

		defer func() {
			room.Msgchan <- Message{
				Text: fmt.Sprintf("--> | %s has left %s.\n", client.Nickname, room.Name),
			}
			log.Printf("Connection From %v closed", c.RemoteAddr())
			room.Rmchan <- client
		}()

		go client.WriteLinesFrom(client.Ch)
		if len(room.Clients) == 3 {
			startMsg := "* -- | Everyone has arrived, mission starting...\n" +
				"* -- | Ask for /help to get familiar around here\n"
			room.Msgchan <- Message{Text: startMsg}
			// Glenda always shows up last
			conn, err := net.Dial("tcp", "127.0.0.1:6000")
			if err != nil {
				log.Printf("Glenda could not connect")
			}
			glenda := Client{
				Conn:     conn,
				Nickname: "Glenda",
				Ch:       make(chan Message),
			}
			room.Addchan <- &glenda
		}
		client.ReadLinesInto(room.Msgchan)
	} else {
		lateMsg := "It seems your teammates have started without you. Find better friends\n"
		io.WriteString(c, lateMsg)
	}
}

func handleIO(r *Room) {
	// handle all io From Clients
	for {
		select {
		case msg := <-r.Msgchan:
			msgText := strings.Trim(strings.Replace(msg.Text, "\n", "", -1), " ")
			first := strings.Split(msgText, " ")[0]
			switch first {
			case "/help":
				help(msg.From.Ch)
			case "/look":
				look(msg.From.Ch, r.Clients)
			default:
				for _, client := range r.Clients {
					client.Ch <- msg
				}
			}
		case client := <-r.Addchan:
			log.Printf("New client: %v", client.Conn)
			r.Clients[client.Conn] = client
		case client := <-r.Rmchan:
			log.Printf("Client disconnects: %v", client.Conn)
			delete(r.Clients, client.Conn)
		default:
		}
	}
}

func help(ch chan Message) {
	helpText := "help -- |  Usage:\n" +
		"help -- |\n" +
		"help -- |     /[cmd] [arguments]\n" +
		"help -- |\n" +
		"help -- |  Available commands:\n" +
		"help -- |\n" +
		"help -- |    /msg [to] [text]         send message to coworker\n" +
		"help -- |    /list                    look at files you have access to\n" +
		"help -- |    /send [to] [filename]    move file to coworker\n" +
		"help -- |    /look                    show coworkers\n"
	ch <- Message{Text: helpText}
}

func look(ch chan Message, clients map[net.Conn]*Client) {
	lookText := "look -- | You look around at your co-workers' nametages:\n"
	for _, c := range clients {
		lookText += ("look -- | " + c.Nickname + "\n")
	}
	ch <- Message{Text: lookText}
}
