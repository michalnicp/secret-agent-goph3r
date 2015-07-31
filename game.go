package main

import (
	"bufio"
	"fmt"
	"log"
)

type Message struct {
	From string
	To   string
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
	Filechan  chan File
	Files     []File
	isStarted bool
	Failed    bool
	Done      chan bool
}

func GameHandler(clientCh chan *Client) {
	games := make(map[string]*Game)

	for {
		select {
		case client := <-clientCh:
			// Join a game
			gameName, err := Prompt(client.RWC, ROOM_MSG)
			if err != nil {
				log.Printf("Error occured while prompting: %s", err.Error())
				client.Done <- true
				continue
			}
			game, ok := games[gameName]
			if !ok {
				// Create a new game with name
				log.Printf("Creating a new game %s", gameName)
				game = NewGame(gameName)
				games[gameName] = game
				go game.Init()
			}
			// maximum 3 clients per game
			if len(game.Clients) < 3 {
				game.Addchan <- client
			} else {
				// kick the client
				client.Msgchan <- Message{
					To:   client.Nickname,
					Text: "It seems your teammates have started without you...\n",
				}
				client.Close()
			}
		}
	}
}

func NewGame(name string) *Game {
	return &Game{
		Name:      name,
		Clients:   make(map[string]*Client),
		Addchan:   make(chan *Client, 5), // TODO do I really need a buffer chan
		Rmchan:    make(chan Client),
		Filechan:  make(chan File, 5), // TODO do I really need a buffered chan
		isStarted: false,
	}
}

func (g *Game) ClientHandler(done chan bool) {
	for {
		select {
		case client := <-g.Addchan:
			// Check if clients already exist with nickname
			for {
				if _, ok := g.Clients[client.Nickname]; ok {
					// Nickname already exists, get new nickname
					bufw := bufio.NewWriter(client.RWC)
					if _, err := bufw.WriteString(fmt.Sprintf("err -- | Client with nickname \"%s\" already exists. Choose a new nickname.\n", client.Nickname)); err != nil {
						log.Printf("Error occuring while writing: %s\n", err.Error())
					}
					if err := bufw.Flush(); err != nil {
						log.Printf("Error occured while flushing: %s\n", err.Error())
					}

					nickname, err := GetNickname(client.RWC)
					if err != nil {
						log.Printf("Error while getting nickname: %s", err.Error())
					}
					client.Nickname = nickname
				} else {
					break
				}
			}
			log.Printf("New client %s", client.Nickname)
			g.Clients[client.Nickname] = client
			client.Game = g
			log.Printf("Client %s has joined %s", client.Nickname, g.Name)
			log.Printf("Game %s now has %v", g.Name, g.Clients)

			go client.Start()

			g.MsgAll(fmt.Sprintf("--> | %s has joined %s, waiting for teammates...\n", client.Nickname, client.Game.Name))
			if len(g.Clients) == 3 {
				g.Start()
			}
		case client := <-g.Rmchan:
			// TODO cancel game when someone leaves
			log.Printf("Client left: %s", client.Nickname)
			g.MsgAll(fmt.Sprintf("--> | %s has left %s, exiting game...\n", client.Nickname, client.Game.Name))
			delete(g.Clients, client.Nickname)
			g.End()
		}
	}
}

func (g *Game) FileHandler(done chan bool) {
	for {
		select {
		case file := <-g.Filechan:
			log.Printf("Game %s received file %s", g.Name, file.Filename)
			g.Files = append(g.Files, file)
		case <-done:
			return
		}
	}
}

func (g *Game) Init() {
	done := make(chan bool)
	go g.FileHandler(done)
	go g.ClientHandler(done)

	<-g.Done
	if g.Failed {
		for _, c := range g.Clients {
			c.Msgchan <- Message{Text: FAIL_MSG}
			c.Done <- true
		}
	} else {
		// TODO calculate score
		score := 100
		for _, c := range g.Clients {
			c.Msgchan <- Message{Text: fmt.Sprintf("Game ended. Score %d", score)}
			c.Done <- true
		}
	}
	done <- true
	done <- true
}

func (g *Game) Start() {
	log.Printf("Starting game %s", g.Name)
	g.LoadFilesIntoClients()
	g.isStarted = true
	g.MsgAll(START_MSG)
}

func (g *Game) End() {
	g.Done <- true
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
		c.Msgchan <- Message{Text: text}
	}
}

func (g *Game) LoadFilesIntoClients() {
	capacities := []int{50, 81, 120}
	weights := []int{23, 31, 29, 44, 53, 38, 63, 85, 89, 82}
	profits := []int{92, 57, 49, 68, 60, 43, 67, 84, 86, 72}

	totalFiles := 0

	for {
		// iterating over maps is random, no need to use perm
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
