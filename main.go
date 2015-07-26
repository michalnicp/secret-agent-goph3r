package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
)

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
	portNumber := os.Getenv("PORT")

	if portNumber == "" {
		portNumber = "6000"
	}

	port, err := strconv.Atoi(portNumber)
	if err != nil {
		log.Fatalf("An error occured parsing %s to integer: %s\n", portNumber, err.Error())
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rooms := make(map[string]*Room)

	log.Printf("Now accepting clients on port %d\n", port)

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
		Files:     make([]File, 0),
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

			for clientName, client := range room.Clients {
				fmt.Printf("In room %s we have client %s at address %p.\n", room.Name, clientName, client)
			}

			loadFilesClients(room.Clients)
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
			log.Printf("New client: %p", client)
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

func loadFilesClients(clients map[string]*Client) {
	//f, err := os.Open("dataset.txt")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer f.Close()

	//r := bufio.NewReader(f)

	//line, _ := r.ReadString('\n')
	//cdata := strings.Split(line, ",")
	//capacities := make([]int, 0)
	//for _, val := range cdata {
	//	c, _ := strconv.Atoi(val)
	//	capacities = append(capacities, c)
	//}

	//line, _ = r.ReadString('\n')
	//wdata := strings.Split(line, ",")
	//weights := make([]int, 0)
	//for _, val := range wdata {
	//	w, _ := strconv.Atoi(val)
	//	weights = append(weights, w)
	//}

	//line, _ = r.ReadString('\n')
	//pdata := strings.Split(line, ",")
	//profits := make([]int, 0)
	//for _, val := range pdata {
	//	p, _ := strconv.Atoi(val)
	//	profits = append(profits, p)
	//}
	//if len(weights) != len(profits) {
	//	log.Fatal("Number of weights and profits not equal")
	//}
	capacities := []int{50, 81, 120}
	weights := []int{23, 31, 29, 44, 53, 38, 63, 85, 89, 82}
	profits := []int{92, 57, 49, 68, 60, 43, 67, 84, 86, 72}

	// Shuffle data
	weights = ShuffleInt(weights)
	profits = ShuffleInt(profits)

	totalFiles := 0

	for {

		for _, client := range clients {

			file := File{
				Filename: fmt.Sprintf("filename_%d.txt", totalFiles),
				Size:     weights[totalFiles],
				Secrecy:  profits[totalFiles],
			}

			fmt.Printf("Adding %s file to client %p with files pointer %p\n", file.Filename, client, client.Files)
			client.Files = append(client.Files, file)

			totalFiles++

			if totalFiles >= 10 {
				break
			}
		}

		if totalFiles >= 10 {
			break
		}
	}

	i := 0
	for _, client := range clients {
		client.Bandwidth = capacities[i]
		i++
	}

}

func ShuffleInt(list []int) []int {
	shuffledList := make([]int, len(list))
	perm := rand.Perm(len(list))

	for i, v := range list {
		shuffledList[perm[i]] = v
	}
	return shuffledList
}
