package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"net"
	"regexp"
	"time"
)

const (
	LOBBY = iota
	RUNNING
	EXIT
	FAIL
)

const MAX_NUM_CLIENTS int = 3
const TIMEOUT int = 60

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
	Name       string
	Clients    map[string]*Client
	DoneClient chan bool
	AddCh      chan *Client
	RmCh       chan *Client
	MsgCh      chan Message
	FileCh     chan File
	Files      []File
	Score      int
	Status     int
}

type GameRequest struct {
	Name string
	Ch   chan *Game // Channel on which to send game back to requester
}

func NewGame(name string) *Game {
	return &Game{
		Name:       name,
		Clients:    make(map[string]*Client),
		DoneClient: make(chan bool, MAX_NUM_CLIENTS),
		AddCh:      make(chan *Client, MAX_NUM_CLIENTS),
		RmCh:       make(chan *Client, MAX_NUM_CLIENTS),
		MsgCh:      make(chan Message),
		FileCh:     make(chan File, 5), // TODO do I really need a buffered chan
		Files:      make([]File, 0),
		Score:      0,
		Status:     LOBBY,
	}
}

func ConnectionHandler(connCh chan net.Conn, gameRequestCh chan GameRequest) {
	for conn := range connCh {
		client := NewClient(conn)
		if err := client.WriteString(INTRO_MSG); err != nil {
			log.Printf("Error occured while writing: %s", err.Error())
			continue
		}
		go JoinGame(client, gameRequestCh)
	}
}

func JoinGame(client *Client, gameRequestCh chan GameRequest) {
	// Get game name from client. Send request for game and then join it
	bufw := bufio.NewWriter(client.RWC)
	re := regexp.MustCompile(`^\w+$`)
	for {
		gameName, err := client.Prompt(ROOM_MSG)
		if err != nil {
			log.Printf("Error occured while getting room: %s", err.Error())
			return
		}

		gameName = re.FindString(gameName)
		if gameName == "" {
			if _, err := bufw.WriteString("Invalid channel\n"); err != nil {
				log.Printf("Error occuring while writing: %s\n", err.Error())
				return
			}
			if err := bufw.Flush(); err != nil {
				log.Printf("Error occured while flushing: %s\n", err.Error())
				return
			}
			continue
		}
		ch := make(chan *Game)
		gameRequestCh <- GameRequest{
			Name: gameName,
			Ch:   ch,
		}
		game := <-ch
		game.AddCh <- client
		return
	}
}

func GameHandler(requestCh chan GameRequest) {
	// Handles all requests for games. Creates new games it they do not exist and starts them
	games := make(map[string]*Game)
	done := make(chan *Game)

	for {
		select {
		case request := <-requestCh:
			gameName := request.Name
			game, ok := games[gameName]
			if !ok {
				// Create a new game with name
				log.Printf("Creating a new game \"%s\"", gameName)
				game = NewGame(gameName)
				games[gameName] = game
				go game.Start(done)
			}
			request.Ch <- game
		case game := <-done:
			log.Printf("Delete game \"%s\"", game.Name)
			delete(games, game.Name)
		}
	}
}

func (g *Game) Start(done chan *Game) {
	log.Printf("Starting game %s", g.Name)
	go g.FileHandler()
	go g.ClientHandler()
	go g.MsgHandler()

	numDone := 0
loop:
	for {
		select {
		case <-time.After(time.Duration(TIMEOUT) * time.Second):
			log.Printf("Game %s has timed out", g.Name)
			g.Status = FAIL
			break loop
		case <-g.DoneClient:
			numDone += 1
			if numDone >= MAX_NUM_CLIENTS {
				break loop
			}
		}
	}

	switch g.Status {
	case EXIT:
		g.MsgAll(LEFT_MSG)
	case FAIL:
		g.MsgAll(FAIL_MSG)
	case RUNNING:
		// TODO calculate score
		scoreText := fmt.Sprintf("Game ended. Score %d\n", g.Score)
		g.MsgAll(scoreText)
	}
	log.Printf("Ending game \"%s\"", g.Name)
	g.EndClients()
	done <- g
}

func (g *Game) Init() {
	log.Printf("Initializing game %s", g.Name)
	g.Status = RUNNING
	g.LoadFiles()
	g.MsgAll(START_MSG)
}

func (g *Game) End(status int) {
	// TODO is this all I need to do?
	g.Status = status
	//close(g.FileCh)
	//close(g.MsgCh)
	for i := 0; i < MAX_NUM_CLIENTS; i++ {
		g.DoneClient <- true
	}
}

func (g *Game) EndClients() {
	for _, c := range g.Clients {
		c.End()
	}
}

func (g *Game) ClientHandler() {
	for {
		select {
		case client := <-g.AddCh:
			// maximum 3 clients per game
			if len(g.Clients) >= MAX_NUM_CLIENTS {
				// Kick the Client
				if err := client.WriteString(FULL_MSG); err != nil {
					log.Printf("Error occured while writing full msg: %s", err.Error())
				}
				client.End()
				continue
			}

			// Get name
			// TODO do this asynchronously
			name, err := client.GetName()
			if err != nil {
				log.Printf("Error getting name: %s", err.Error())
				client.End()
				continue
			}
			client.Name = name

			// Check if clients already exist with name
			if _, ok := g.Clients[name]; ok {
				log.Printf("Error name \"%s\" taken", name)
				client.WriteString("Error name taken.\n")
				client.End()
				continue
			}

			g.Clients[client.Name] = client
			client.Game = g
			client.Start()
			log.Printf("New player \"%s\" has joined game \"%s\"", client.Name, g.Name)
			g.MsgAll(fmt.Sprintf("--> | %s has joined %s, waiting for teammates...\n", client.Name, client.Game.Name))
			if len(g.Clients) == MAX_NUM_CLIENTS {
				g.Init()
			}
		case client := <-g.RmCh:
			// TODO cancel game when someone leaves
			log.Printf("Player \"%s\" has left game \"%s\"", client.Name, g.Name)
			delete(g.Clients, client.Name)
			g.MsgAll(fmt.Sprintf("--> | %s has left %s\n", client.Name, client.Game.Name))
			g.End(EXIT)
		}
	}
}

func (g *Game) FileHandler() {
	for file := range g.FileCh {
		log.Printf("Game %s received file %s", g.Name, file.Filename)
		g.Files = append(g.Files, file)
		g.Score += file.Secrecy
	}
}

func (g *Game) MsgHandler() {
	for msg := range g.MsgCh {
		from := g.Clients[msg.From]
		if msg.To == "Glenda" {
			if msg.Text == "done" {
				from.DoneSendingFiles = true
				g.DoneClient <- true
				continue
			} else {
				from.MsgCh <- Message{Text: GLENDA_MSG}
				continue
			}
		}
		to, ok := g.Clients[msg.To]
		if ok {
			msg.Text = fmt.Sprintf("%s | %s\n", msg.From, msg.Text)
			to.MsgCh <- msg
		} else {
			from.MsgCh <- Message{Text: fmt.Sprintf("err -- | Client \"%s\" does not exist\n", msg.To)}
		}
	}
}

func (g *Game) MsgAll(text string) {
	msg := Message{Text: text}
	for _, c := range g.Clients {
		c.MsgCh <- msg
	}
}

func (g *Game) LoadFiles() error {
	capacities := []int{50, 81, 120}
	files := GenerateFiles()

	totalFiles := 0
outer:
	for {
		// TODO randomize order
		for _, c := range g.Clients {
			c.Files = append(c.Files, files[totalFiles])
			totalFiles++
			if totalFiles >= 10 {
				break outer
			}
		}
	}

	i := 0
	for _, c := range g.Clients {
		c.Bandwidth = capacities[i]
		i++
	}
	return nil
}

func GenerateFiles() []File {
	files := make([]File, 0)
	filenames := []string{"top_secret.txt", "contacts.csv", "banknotes.dat", "cats.png", "notes_201501105.md", "secrets.ppt",
		"jokes.txt", "pie_graph.png", "bar_chart.xcl", "peer_review_hilarious.txt", "instant_soup.txt", "dilbert_comics.jpg",
		"screenshots.jpg", "logo.png"}
	ShuffleStrings(filenames)
	weights := []int{23, 31, 29, 44, 53, 38, 63, 85, 89, 82}
	profits := []int{92, 57, 49, 68, 60, 43, 67, 84, 86, 72}

	for i := 0; i < 10; i++ {
		f := File{
			Filename: filenames[i],
			Size:     weights[i],
			Secrecy:  profits[i],
		}
		files = append(files, f)
	}
	return files
}

func ShuffleStrings(slc []string) {
	rand.Seed(time.Now().UnixNano())
	for i := 1; i < len(slc); i++ {
		r := rand.Intn(i + 1)
		if i != r {
			slc[r], slc[i] = slc[i], slc[r]
		}
	}
}
