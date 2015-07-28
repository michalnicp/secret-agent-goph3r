package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"text/tabwriter"
)

type Client struct {
	Conn             net.Conn
	Nickname         string
	Ch               chan Message
	Files            []File
	Filechan         chan File
	Bandwidth        int
	DoneSendingFiles chan bool
	Done             chan bool
}

func NewClient(c net.Conn) *Client {
	client := Client{
		Conn:     c,
		Ch:       make(chan Message),
		Files:    make([]File, 0),
		Filechan: make(chan File),
		Done:     make(chan bool, 1),
	}

	client.Write(INTRO_MSG)

	// get nickname
	var nickname string
	var err error
	for {
		client.Write("Enter a nickname: ")
		if nickname, err = client.Read(); err != nil {
			return nil
		}
		nickname = strings.TrimSpace(nickname)
		if nickname == "" {
			client.Write("Invalid Username\n")
			continue
		}
		// TODO games to see if name is taken, or autogenerate nickname
		client.Write("Nickname taken\n")
	}
	client.Nickname = nickname

	return &client
}

func (client *Client) Start(msgChan chan Message) {
	go client.WriteLinesFrom(client.Ch)
	go client.ReadLinesInto(msgChan)
	go client.ReceiveFilesFrom(client.Filechan)
}

func (c *Client) ReadLinesInto(ch chan<- Message) {
	reader := bufio.NewReader(c.Conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("An error occured while reading: %s\n", err.Error())
			c.Done <- true
			return
		}
		message := Message{
			From: c,
			Text: line,
		}
		ch <- message
	}
}

func (c Client) WriteLinesFrom(ch <-chan Message) {
	writer := bufio.NewWriter(c.Conn)
	for msg := range ch {
		if _, err := writer.WriteString(msg.Text); err != nil {
			log.Printf("An error occured writing: %s\n", err.Error())
			c.Done <- true
			return
		}
		if err := writer.Flush(); err != nil {
			log.Printf("An error occured flushing: %s\n", err.Error())
			c.Done <- true
			return
		}
	}
}

func (c *Client) ListFiles() {
	io.WriteString(c.Conn, fmt.Sprintf("list -- | Remaining Bandwidth: %d KB\n", c.Bandwidth))
	w := new(tabwriter.Writer)
	w.Init(c.Conn, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "list -- |\tName\tSize\tSecrecy Value")
	for _, file := range c.Files {
		fmt.Fprintf(w, "list -- |\t%s\t%d\t%d\n", file.Filename, file.Size, file.Secrecy)
	}
	w.Flush()
}

func (c *Client) ReceiveFilesFrom(ch <-chan File) {
	writer := bufio.NewWriter(c.Conn)

	for file := range ch {
		if _, err := writer.WriteString(fmt.Sprintf("send -- | Received file: %s\n", file.Filename)); err != nil {
			log.Printf("An error occured writing: %s\n", err.Error())
		}

		if err := writer.Flush(); err != nil {
			log.Printf("An error occured flushing: %s\n", err.Error())
		}

		c.Files = append(c.Files, file)
	}
}

func (c *Client) SendFileTo(filename string, ch chan<- File, external bool) {
	newfiles := make([]File, 0)
	for _, file := range c.Files {
		if file.Filename == filename {
			if external {
				c.Bandwidth -= file.Size
			}
			ch <- file
		} else {
			newfiles = append(newfiles, file)
		}
	}
	// TODO Figure out a better way to cut out an element from an array
	c.Files = newfiles
}

func (c *Client) Write(s string) error {
	writer := bufio.NewWriter(c.Conn)

	if _, err := writer.WriteString(s); err != nil {
		log.Printf("An error occured writing: %s\n", err.Error())
		return err
	}

	if err := writer.Flush(); err != nil {
		log.Printf("An error occured flushing: %s\n", err.Error())
		return err
	}

	return nil
}

func (c *Client) Read() (string, error) {
	reader := bufio.NewReader(c.Conn)
	b, _, err := reader.ReadLine()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) Prompt(question string) (string, error) {
	var ans string
	var err error
	if err = c.Write(question); err != nil {
		return "", err
	}
	if ans, err = c.Read(); err != nil {
		return "", err
	}
	return ans, nil
}
