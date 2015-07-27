package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

func main() {
	ln, err := net.Listen("tcp", ":6000")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	log.Println("Now accepting connections on port 6000")

	games := make(map[string]*Game)

	for {
		Conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go handleConnection(Conn, games)
	}
}

func handleConnection(c net.Conn, games map[string]*Game) {
	reader := bufio.NewReader(c)
	writer := bufio.NewWriter(c)
	defer c.Close()

	// Create Game
	gameName := prompt(reader, writer,
		"A monolithic building appears before you. You have arrived at the office. Try not to act suspicious.\n"+
			"Log in to your team's assigned collaboration channel: ")
	game, ok := games[gameName]
	if !ok {
		// Create a new game with name
		game = NewGame(gameName)
		games[gameName] = game
		go games[gameName].HandleIO()
	}
	if game.IsFull() {
		writer.WriteString("It seems your teammates have started without you. Find better friends\n")
		return
	}

	fmt.Printf("Joined Game(%p)\n", game)

	// Create Client
	var nickname string
	for {
		nickname = strings.TrimSpace(prompt(reader, writer, "Enter a nickname: "))
		if nickname == "" {
			writer.WriteString("Invalid Username\n")
			continue
		}
		_, ok := game.Clients[nickname]
		if !ok {
			// client with nickname doesn't exist, valid
			break
		}
		writer.WriteString("Nickname taken\n")
	}
	client := NewClient(nickname, c)
	client.Welcome()

	game.Addchan <- client
	go client.WriteLinesFrom(client.Ch)
	go client.ReadLinesInto(game.Msgchan)
	go client.ReceiveFilesFrom(client.Filechan)
	for _, c := range game.Clients {
		c.Ch <- Message{
			Text: fmt.Sprintf("--> | %s has joined %s, waiting for teammates...\n", client.Nickname, game.Name),
		}
	}

	defer func() {
		game.Msgchan <- Message{
			Text: fmt.Sprintf("/msg all --> | %s has left %s.\n", client.Nickname, game.Name),
		}
		log.Printf("Connection from %v closed", client.Conn.RemoteAddr())
		game.Rmchan <- *client
	}()

	if game.IsFull() {
		game.Start()
	}
	// Wait for done
	<-client.Done
}
