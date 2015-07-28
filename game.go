package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"regexp"
)

type Message struct {
	From *Client
	Text string
}

type File struct {
	Filename string
	Size     int
	Secrecy  int
}

type Game struct {
	Name      string
	Clients   map[string]*Client
	Addchan   chan *Client
	Rmchan    chan Client
	Msgchan   chan Message
	Filechan  chan File
	isStarted bool
	Files     []File
	Done      chan bool
}

func prompt(reader *bufio.Reader, writer *bufio.Writer, question string) string {
	if _, err := writer.WriteString(question); err != nil {
		log.Printf("An error occured writing: %s\n", err.Error())
	}

	if err := writer.Flush(); err != nil {
		log.Printf("An error occured flushing: %s\n", err.Error())
	}

	ans, _, err := reader.ReadLine()
	if err != nil {
		log.Printf("An error occured reading: %s\n", err.Error())
	}

	return string(ans)
}

func NewGame(name string) *Game {
	return &Game{
		Name:      name,
		Clients:   make(map[string]*Client),
		Addchan:   make(chan *Client),
		Rmchan:    make(chan Client),
		Msgchan:   make(chan Message),
		Filechan:  make(chan File, 5), // TODO do I really need a buffered chan
		isStarted: false,
	}
}

func (g *Game) HandleIO() {
	// handle all io From Clients
	re := regexp.MustCompile(`(?s)(\/\w+)\s(.*)`)
	for {
		select {
		case msg := <-g.Msgchan:
			reResult := re.FindStringSubmatch(msg.Text)
			command := reResult[1]
			if !g.isStarted {
				continue
			}
			switch command {
			case "/help":
				g.help(msg)
			case "/look":
				g.look(msg)
			case "/msg":
				g.SendMsg(msg)
				if g.CheckDone() {
					g.End()
				}
			case "/list":
				msg.From.ListFiles()
			case "/send":
				g.SendFile(msg)
			default:
			}
		case client := <-g.Addchan:
			log.Printf("New client: %p", client)
			g.Clients[client.Nickname] = client
		case client := <-g.Rmchan:
			log.Printf("Client disconnects: %v", client.Conn)
			delete(g.Clients, client.Nickname)
		case file := <-g.Filechan:
			log.Printf("recieved file from filechan")
			g.Files = append(g.Files, file)
		case <-g.Done:
			return
		default:
		}
	}
}

func (g *Game) Start() {
	g.loadFilesIntoClients()
	g.isStarted = true
	g.Msgchan <- Message{Text: START_MSG}
}

func (g *Game) End() {
	g.MsgAll("Game has ended\n")
	for _, c := range g.Clients {
		select {
		case c.Done <- true:
		default:
		}
	}
	select {
	case g.Done <- true:
	default: //Error handling here
	}
}

func (g *Game) IsFull() bool {
	return len(g.Clients) >= 3
}

func (g *Game) CheckDone() bool {
	for _, c := range g.Clients {
		if !c.DoneSendingFiles {
			return false
		}
	}
	return true
}

func (g *Game) MsgAll(text string) {
	for _, c := range g.Clients {
		c.Ch <- Message{Text: text}
	}
}

func (g *Game) SendMsg(msg Message) {
	re := regexp.MustCompile(`(?s)\/\w+\s(\w+)\s(.*)`)
	reResult := re.FindStringSubmatch(msg.Text)
	if reResult == nil {
		msg.From.Ch <- Message{Text: "Invalid command. Check /help for usage.\n"}
		return
	}
	to := reResult[1]
	text := reResult[2]
	if to == "Glenda" {
		if text == "done\n" {
			msg.From.DoneSendingFiles = true
			doneText := fmt.Sprintf("-- | %s has finished sending files. Waiting for teammates to finish...\n", msg.From.Nickname)
			g.MsgAll(doneText)
		} else {
			msgGlenda(msg.From.Ch)
		}
	} else if to == "all" {
		g.MsgAll(text)
	} else if c, ok := g.Clients[to]; ok {
		c.Ch <- Message{From: msg.From, Text: fmt.Sprintf("%s | %s", msg.From.Nickname, text)}
	} else {
		msg.From.Ch <- Message{Text: fmt.Sprintf("There is no one here named %s.\n", to)}
	}
}

func (g *Game) SendFile(msg Message) {
	re := regexp.MustCompile(`\/\w+\s(\w+)\s(.*)`)
	reResult := re.FindStringSubmatch(msg.Text)
	if reResult == nil {
		msg.From.Ch <- Message{Text: "Invalid command. Check /help for usage.\n"}
		return
	}
	to := reResult[1]
	filename := reResult[2]
	if to == "Glenda" {
		msg.From.SendFileTo(filename, g.Filechan, true)
		msg.From.Ch <- Message{Text: fmt.Sprintf("send -- | Sent File: %s to Glenda\n", filename)}
		if msg.From.Bandwidth < 0 {
			g.Fail()
		}
	} else if c, ok := g.Clients[to]; ok {
		msg.From.SendFileTo(filename, c.Filechan, false)
		msg.From.Ch <- Message{Text: fmt.Sprintf("send -- | Sent File: %s to %s\n", filename, c.Nickname)}
	} else {
		msg.From.Ch <- Message{Text: fmt.Sprintf("send -- | There is no one here named %s\n", to)}
	}
}

func (g *Game) Fail() {
	for _, c := range g.Clients {
		c.Ch <- Message{Text: FAIL_MSG}
		c.Done <- true
	}
}

func (g *Game) help(msg Message) {
	msg.From.Ch <- Message{Text: HELP_MSG}
}

func (g *Game) look(msg Message) {
	lookText := "look -- | You look around at your co-workers' nametages:\n"
	for _, c := range g.Clients {
		lookText += ("look -- | " + c.Nickname + "\n")
	}
	lookText += "look -- | Glenda\n"
	msg.From.Ch <- Message{Text: lookText}
}

func msgGlenda(ch chan Message) {
	ch <- Message{Text: GLENDA_MSG}
}

func (g *Game) loadFilesIntoClients() {
	capacities := []int{50, 81, 120}
	weights := []int{23, 31, 29, 44, 53, 38, 63, 85, 89, 82}
	profits := []int{92, 57, 49, 68, 60, 43, 67, 84, 86, 72}

	totalFiles := 0

	for {
		for _, client := range g.Clients {
			file := File{
				Filename: fmt.Sprintf("filename_%d.txt", totalFiles),
				Size:     weights[totalFiles],
				Secrecy:  profits[totalFiles],
			}

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
	for _, client := range g.Clients {
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
