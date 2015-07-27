package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"text/tabwriter"
)

type Client struct {
	Conn             net.Conn
	Nickname         string
	Ch               chan Message
	Files            []File
	Filechan         chan File
	Bandwidth        int
	DoneSendingFiles bool
	Done             chan bool
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
	for file := range ch {
		io.WriteString(c.Conn, fmt.Sprintf("send -- | Received file: %s\n", file.Filename))
		c.Files = append(c.Files, file)
	}
}

func (c *Client) SendFileTo(filename string, ch chan File, external bool) {
	newfiles := make([]File, 0)
	for _, file := range c.Files {
		if file.Filename == filename {
			if external {
				c.Bandwidth -= file.Size
			}
			log.Printf("before blocking")
			ch <- file
			log.Printf("after blocking")
		} else {
			newfiles = append(newfiles, file)
		}
	}
	// TODO Figure out a better way to cut out an element from an array
	c.Files = newfiles
}

func NewClient(name string, c net.Conn) *Client {
	return &Client{
		Conn:      c,
		Nickname:  name,
		Ch:        make(chan Message),
		Files:     make([]File, 0),
		Filechan:  make(chan File),
		Bandwidth: 0,
		Done:      make(chan bool, 1),
	}
}

func (c Client) Welcome() {
	writer := bufio.NewWriter(c.Conn)
	welcomeMsg := `A monolithic building appears before you. You have arrived
at the office. Try not to act suspicious.`
	writer.WriteString(string(welcomeMsg))
}
