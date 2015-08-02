package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

const (
	RUNNING = iota
	EXIT
	FAIL
)

const MAX_NUM_CLIENTS int = 3

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
	Name     string
	Clients  map[string]*Client
	AddChan  chan *Client
	RmChan   chan *Client
	FileChan chan File
	Files    []File
	Status   int
	WG       *sync.WaitGroup
}

func GameHandler(ch chan net.Conn) {
	games := make(map[string]*Game)

	for conn := range ch {
		go InitClient(conn, games)
	}
}

func NewGame(name string) *Game {
	var wg sync.WaitGroup
	return &Game{
		Name:     name,
		Clients:  make(map[string]*Client),
		AddChan:  make(chan *Client, 5), // TODO do I really need a buffer chan
		RmChan:   make(chan *Client),
		FileChan: make(chan File, 5), // TODO do I really need a buffered chan
		Files:    make([]File, 0),
		Status:   RUNNING,
		WG:       &wg,
	}
}

func InitClient(conn net.Conn, games map[string]*Game) {
	var err error
	defer func() {
		// If any errors occured, close the connection
		if err != nil {
			log.Printf("Closing connection %s", conn.RemoteAddr())
			conn.Close()
		}
	}()

	log.Printf("New connection from %s", conn.RemoteAddr())
	if _, err := conn.Write([]byte(INTRO_MSG)); err != nil {
		log.Printf("Error occured while writing: %s", err.Error())
		return
	}

	// Join/Create a game
	gameName, err := Prompt(conn, ROOM_MSG)
	if err != nil {
		log.Printf("Error occured while getting room: %s", err.Error())
		return
	}
	game, ok := games[gameName]
	if !ok {
		// Create a new game with name
		// TODO request game instead
		log.Printf("Creating a new game %s", gameName)
		game = NewGame(gameName)
		games[gameName] = game
		go game.ClientHandler()
	}

	// maximum 3 clients per game
	if len(game.Clients) < MAX_NUM_CLIENTS {
		// Get nickname
		var nickname string
		for {
			nickname, err = GetNickname(conn)
			if err != nil {
				log.Printf("Error getting nickname: %s", err.Error())
				return
			}
			if _, ok := game.Clients[nickname]; ok {
				// nickname already exists in game
				if _, err := conn.Write([]byte("Error nickname taken.\n")); err != nil {
					log.Printf("Error occured while writing: %s", err.Error())
					return
				}
				continue
			}
			break
		}
		client := NewClient(conn, nickname)
		game.AddChan <- client
	} else {
		// Close the connection
		if _, err := conn.Write([]byte(FULL_MSG)); err != nil {
			log.Printf("Error occured while writing: %s", err.Error())
			return
		}
		log.Printf("Closing connection %s", conn.RemoteAddr())
		conn.Close()
		return
	}

	if len(game.Clients) == MAX_NUM_CLIENTS {
		go game.Start()
	}
}

func (g *Game) Start() {
	log.Printf("Starting game %s", g.Name)
	go g.FileHandler()
	LoadFilesIntoClients(g.Clients)
	g.Status = RUNNING
	MsgAll(START_MSG, g.Clients)

	for _, c := range g.Clients {
		g.WG.Add(1)
		go c.Start()
	}
	g.WG.Wait()

	switch g.Status {
	case EXIT:
		for _, c := range g.Clients {
			c.MsgChan <- Message{Text: "One of your teamates chickent out. Ending game..."}
		}
	case FAIL:
		for _, c := range g.Clients {
			c.MsgChan <- Message{Text: FAIL_MSG}
		}
	case RUNNING:
		// TODO calculate score
		score := 100
		MsgAll(fmt.Sprintf("Game ended. Score %d\n", score), g.Clients)
		g.KickClients("")
	}
}

func (g *Game) End() {
	close(g.FileChan)
}

func (g *Game) ClientHandler() {
	for {
		select {
		case client := <-g.AddChan:
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

			WriteStringToClients(fmt.Sprintf("--> | %s has joined %s, waiting for teammates...\n", client.Nickname, client.Game.Name), g.Clients)
			log.Printf("line 201")
			if len(g.Clients) == MAX_NUM_CLIENTS {
				g.Start()
			}
		case client := <-g.RmChan:
			// TODO cancel game when someone leaves
			log.Printf("%s left %s", client.Nickname, g.Name)
			g.Status = EXIT
			g.KickClients(fmt.Sprintf("--> | %s has left %s, exiting game...\n", client.Nickname, client.Game.Name))
			g.End()
		}
	}
}

func (g *Game) FileHandler() {
	for file := range g.FileChan {
		log.Printf("Game %s received file %s", g.Name, file.Filename)
		g.Files = append(g.Files, file)
	}
}

func (g *Game) CheckDone() bool {
	for _, c := range g.Clients {
		if !c.DoneSendingFiles {
			return false
		}
	}
	return true
}

func (g *Game) KickClients(text string) {
	for _, c := range g.Clients {
		c.RWC.Write([]byte(text))
		if !c.DoneSendingFiles {
			c.DoneSendingFiles = true
			g.WG.Done()
		}
		c.End()
	}
}

func MsgAll(text string, clients map[string]*Client) {
	for _, c := range clients {
		c.MsgChan <- Message{Text: text}
	}
}

func WriteStringToClients(text string, clients map[string]*Client) error {
	var err error
	for _, c := range clients {
		bufw := bufio.NewWriter(c.RWC)
		if _, err = bufw.WriteString(text); err != nil {
			log.Printf("Error occuring while writing: %s\n", err.Error())
			return err
		}
		if err = bufw.Flush(); err != nil {
			log.Printf("Error occured while flushing: %s\n", err.Error())
			return err
		}
	}
	return nil
}

func LoadFilesIntoClients(clients map[string]*Client) error {
	if len(clients) < MAX_NUM_CLIENTS {
		return errors.New("not enough clients")
	}

	capacities := []int{50, 81, 120}
	weights := []int{23, 31, 29, 44, 53, 38, 63, 85, 89, 82}
	profits := []int{92, 57, 49, 68, 60, 43, 67, 84, 86, 72}

	totalFiles := 0

	for {
		// iterating over maps is random, no need to use perm
		for _, c := range clients {
			file := File{
				Filename: fmt.Sprintf("filename_%d.txt", totalFiles),
				Size:     weights[totalFiles],
				Secrecy:  profits[totalFiles],
			}

			c.Files = append(c.Files, file)

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
	for _, c := range clients {
		c.Bandwidth = capacities[i]
		i++
	}
	return nil
}
