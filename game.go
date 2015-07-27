package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"strings"
)

type Glenda struct {
	Name     string
	Filechan chan File
}

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
	isStarted bool
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
		isStarted: false,
	}
}

func (g *Game) HandleIO() {
	// handle all io From Clients
	for {
		select {
		case msg := <-g.Msgchan:
			msgText := strings.TrimSpace(msg.Text)
			splitText := strings.SplitN(msgText, " ", 2)
			if !g.isStarted {
				continue
			}
			switch splitText[0] {
			case "/help":
				help(msg.From.Ch)
			case "/look":
				look(msg.From.Ch, g.Clients)
			case "/msg":
				splitText = strings.SplitN(splitText[1], " ", 2)
				to := splitText[0]
				text := splitText[1] + "\n"
				if to == "Glenda" {
					msgGlenda(msg.From.Ch)
				} else if to == "all" {
					for _, c := range g.Clients {
						c.Ch <- Message{Text: text}
					}
				} else if c, ok := g.Clients[to]; ok {
					c.Ch <- Message{From: msg.From, Text: fmt.Sprintf("%s | %s\n", msg.From.Nickname, text)}
				} else {
					msg.From.Ch <- Message{Text: fmt.Sprintf("There is no one here named %s.\n", to)}
				}
			case "/list":
				msg.From.ListFiles()
			case "/send":
				log.Println(splitText)
				splitText = strings.SplitN(splitText[1], " ", 2)
				to := splitText[0]
				filename := splitText[1]
				if c, ok := g.Clients[to]; ok {
					msg.From.SendFileTo(filename, c.Filechan)
				}
			default:
			}
		case client := <-g.Addchan:
			log.Printf("New client: %p", client)
			g.Clients[client.Nickname] = client
		case client := <-g.Rmchan:
			log.Printf("Client disconnects: %v", client.Conn)
			delete(g.Clients, client.Nickname)
		default:
		}
	}
}

func (g *Game) Start() {
	g.loadFilesIntoClients()
	g.isStarted = true
	startMsg := "/msg all * -- | Everyone has arrived, mission starting...\n" +
		"* -- | Ask for /help to get familiar around here\n"
	g.Msgchan <- Message{Text: startMsg}
}

func (g *Game) IsFull() bool {
	return len(g.Clients) >= 3
}

func help(ch chan Message) {
	helpText := `help -- |  Usage:
help -- |
help -- |     /[cmd] [arguments]
help -- |
help -- |  Available commands:
help -- |
help -- |    /msg [to] [text]         send message to coworker
help -- |    /list                    look at files you have access to
help -- |    /send [to] [filename]    move file to coworker
help -- |    /look                    show coworkers`
	ch <- Message{Text: string(helpText)}
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
	glendaText := `Glenda | Psst, hey there. I'm going to need your help if we want to exfiltrate
Glenda | these documents. You have clearance that I don't.
Glenda |
Glenda | You each have access to a different set of sensitive files. Within your
Glenda | group you can freely send files to each other for further analysis.
Glenda | However, when sending files to me, the corporate infrastructure team
Glenda | will be alerted if you exceed your transfer quota. Working on too many
Glenda | files will make them suspicious.
Glenda |
Glenda | Please optimize your transfers by the political impact it will create
Glenda | without exceeding any individual transfer quota. The file's security
Glenda | clearance is a good metric to go by for that. Thanks!
Glenda |
Glenda | When each of you is finished sending me files, send me the message
Glenda | 'done'. I'll wait to hear this from all of you before we execute phase
Glenda | two.`
	ch <- Message{Text: string(glendaText)}
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

	for _, client := range g.Clients {
		fmt.Printf("Client(%p).Files(%p) = %#v\n", client, client.Files, client.Files)
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
