package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"text/tabwriter"
)

type Client struct {
	Conn      net.Conn
	Nickname  string
	Ch        chan Message
	Files     []File
	Filechan  chan File
	Bandwidth int
	Done      bool
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
		_, err := io.WriteString(c.Conn, msg.Text)
		if err != nil {
			return
		}
	}
}

func (c Client) ListFiles() {
	io.WriteString(c.Conn, fmt.Sprintf("list -- | Remaining Bandwidth: %d KB\n", c.Bandwidth))
	w := new(tabwriter.Writer)
	w.Init(c.Conn, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "list -- |\tName\tSize\tSecrecy Value")
	for _, file := range c.Files {
		fmt.Fprintf(w, "list -- |\t%s\t%d\t%d\n", file.Filename, file.Size, file.Secrecy)
	}
	w.Flush()
}

type Message struct {
	From Client
	Text string
}

type Room struct {
	Name    string
	Clients map[string]*Client
	Addchan chan *Client
	Rmchan  chan Client
	Msgchan chan Message
}

type Glenda struct {
	Name     string
	Filechan chan File
}

type File struct {
	Filename string
	Size     int
	Secrecy  int
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

	roomName := promptRoom(c, bufc)
	room, ok := rooms[roomName]
	if !ok {
		// Create a new room with name
		room = &Room{
			Name:    roomName,
			Clients: make(map[string]*Client, 0),
			Addchan: make(chan *Client),
			Rmchan:  make(chan Client),
			Msgchan: make(chan Message),
		}
		rooms[roomName] = room
		go handleIO(rooms[roomName])
	}

	client := Client{
		Conn:      c,
		Ch:        make(chan Message),
		Files:     generateFiles(),
		Filechan:  make(chan File),
		Bandwidth: 1000,
		Done:      false,
	}
	for {
		nickname := strings.TrimSpace(promptNick(c, bufc))
		if nickname == "" {
			io.WriteString(c, "Invalid Username\n")
			continue
		}
		_, ok := room.Clients[nickname]
		if !ok {
			// client with nickname doesn't exits
			client.Nickname = nickname
			break
		}
		io.WriteString(c, "Nickname taken\n")
	}

	welcomeMsg := "A monolithic building appears before you. You have arrived\n" +
		"at the office. Try not to act suspicious.\n\n"
	io.WriteString(c, welcomeMsg)

	if len(room.Clients) < 3 {
		room.Addchan <- &client
		room.Msgchan <- Message{
			Text: fmt.Sprintf("/msg all --> | %s has joined %s, waiting for teammates...\n", client.Nickname, room.Name),
		}

		defer func() {
			room.Msgchan <- Message{
				Text: fmt.Sprintf("/msg all --> | %s has left %s.\n", client.Nickname, room.Name),
			}
			log.Printf("Connection From %v closed", c.RemoteAddr())
			room.Rmchan <- client
		}()

		go client.WriteLinesFrom(client.Ch)
		if len(room.Clients) == 3 {
			startMsg := "/msg all * -- | Everyone has arrived, mission starting...\n" +
				"* -- | Ask for /help to get familiar around here\n"
			room.Msgchan <- Message{Text: startMsg}
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
			msgText := strings.TrimSpace(msg.Text)
			splitText := strings.SplitN(msgText, " ", 2)
			switch splitText[0] {
			case "/help":
				help(msg.From.Ch)
			case "/look":
				look(msg.From.Ch, r.Clients)
			case "/msg":
				splitText = strings.SplitN(splitText[1], " ", 2)
				to := splitText[0]
				text := splitText[1] + "\n"
				if to == "Glenda" {
					msgGlenda(msg.From.Ch)
				} else if to == "all" {
					for _, c := range r.Clients {
						c.Ch <- Message{Text: text}
					}
				} else if c, ok := r.Clients[to]; ok {
					c.Ch <- Message{From: msg.From, Text: fmt.Sprintf("%s | %s\n", msg.From.Nickname, text)}
				} else {
					msg.From.Ch <- Message{Text: fmt.Sprintf("There is no one here named %s.\n", to)}
				}
			case "/list":
				log.Printf("listing files")
				msg.From.ListFiles()
			default:
			}
		case client := <-r.Addchan:
			log.Printf("New client: %v", client.Conn)
			r.Clients[client.Nickname] = client
		case client := <-r.Rmchan:
			log.Printf("Client disconnects: %v", client.Conn)
			delete(r.Clients, client.Nickname)
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

func look(ch chan Message, clients map[string]*Client) {
	lookText := "look -- | You look around at your co-workers' nametages:\n"
	for _, c := range clients {
		lookText += ("look -- | " + c.Nickname + "\n")
	}
	lookText += "look -- | Glenda\n"
	ch <- Message{Text: lookText}
}

func msgGlenda(ch chan Message) {
	glendaText := "Glenda | Psst, hey there. I'm going to need your help if we want to exfiltrate\n" +
		"Glenda | these documents. You have clearance that I don't.\n" +
		"Glenda |\n" +
		"Glenda | You each have access to a different set of sensitive files. Within your\n" +
		"Glenda | group you can freely send files to each other for further analysis.\n" +
		"Glenda | However, when sending files to me, the corporate infrastructure team\n" +
		"Glenda | will be alerted if you exceed your transfer quota. Working on too many\n" +
		"Glenda | files will make them suspicious.\n" +
		"Glenda |\n" +
		"Glenda | Please optimize your transfers by the political impact it will create\n" +
		"Glenda | without exceeding any individual transfer quota. The file's security\n" +
		"Glenda | clearance is a good metric to go by for that. Thanks!\n" +
		"Glenda |\n" +
		"Glenda | When each of you is finished sending me files, send me the message\n" +
		"Glenda | 'done'. I'll wait to hear this from all of you before we execute phase\n" +
		"Glenda | two.\n"
	ch <- Message{Text: glendaText}
}

func generateFiles() []File {
	files := make([]File, 15)
	for i, _ := range files {
		//resp, err := http.Get("http://randomword.setgetgo.com/get.php")
		//if err != nil {
		//log.Fatal(err)
		//}
		//filename, err := ioutil.ReadAll(resp.Body)
		//resp.Body.Close()
		//if err != nil {
		//log.Fatal(err)
		//}
		files[i] = File{
			Filename: "filename",
			Size:     10,
			Secrecy:  10,
		}
	}
	log.Printf("%v", files)
	return files
}
