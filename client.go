package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"
	"text/tabwriter"
)

type Client struct {
	RWC              io.ReadWriteCloser
	Nickname         string
	Ch               chan Message
	Files            []File
	Filechan         chan File
	Bandwidth        int
	DoneSendingFiles bool
	Done             chan bool
}

func InitClient(rwc io.ReadWriteCloser, ch chan *Client) {
	bufw := bufio.NewWriter(rwc)

	// Intro message
	if _, err := bufw.WriteString(INTRO_MSG); err != nil {
		log.Printf("Error occured while writing: %s", err.Error())
	}
	if err := bufw.Flush(); err != nil {
		log.Printf("Error occured while flushing: %s\n", err.Error())
	}

	// Get nickname
	var nickname string
	var err error
	for {
		nickname, err = Prompt(rwc, NICK_MSG)
		if err != nil {
			log.Printf("Error occuring while prompting: %s", err.Error())
			return
		}

		nickname = strings.TrimSpace(nickname)
		if nickname == "" {
			if _, err := bufw.WriteString("Invalid Username\n"); err != nil {
				log.Printf("Error occuring while writing: %s\n", err.Error())
			}
			if err := bufw.Flush(); err != nil {
				log.Printf("Error occured while flushing: %s\n", err.Error())
				return
			}
			continue
		}
		// TODO games to see if name is taken, or autogenerate nickname
		//client.Write("Nickname taken\n")
		break
	}
	client := NewClient(rwc, nickname)

	ch <- client
}

func NewClient(rwc io.ReadWriteCloser, nickname string) *Client {
	return &Client{
		RWC:              rwc,
		Nickname:         nickname,
		Ch:               make(chan Message),
		Files:            make([]File, 0),
		Filechan:         make(chan File),
		Bandwidth:        100,
		DoneSendingFiles: false,
		Done:             make(chan bool),
	}
}

func (c *Client) Start(msgChan chan Message) {
	done := make(chan bool)

	go c.WriteLinesFrom(done, c.Ch)
	go c.ReadLinesInto(done, msgChan)
	go c.ReceiveFilesFrom(done, c.Filechan)

	<-c.Done

	done <- true
	done <- true
	done <- true
}

func (c *Client) Close() {
	c.Done <- true
}

func (c *Client) ReadLinesInto(done chan bool, ch chan<- Message) {
	bufr := bufio.NewReader(c.RWC)
	for {
		select {
		case <-done:
			return
		default:
			line, err := bufr.ReadString('\n')
			if err == io.EOF {
				continue
			}
			if err != nil {
				log.Printf("An error occured while reading: %s\n", err.Error())
				return
			}

			message := Message{
				From: c,
				Text: line,
			}
			ch <- message
		}
	}
}

func (c *Client) WriteLinesFrom(done chan bool, ch <-chan Message) {
	bufw := bufio.NewWriter(c.RWC)
	for {
		select {
		case msg := <-ch:
			if _, err := bufw.WriteString(msg.Text); err != nil {
				log.Printf("An error occured writing: %s\n", err.Error())
				c.Close()
				return
			}
			if err := bufw.Flush(); err != nil {
				log.Printf("An error occured flushing: %s\n", err.Error())
				c.Close()
				return
			}
		case <-done:
			return
		}
	}
}

func (c *Client) HasFile(filename string) bool {
	for _, f := range c.Files {
		if f.Filename == filename {
			return true
		}
	}
	return false
}

func (c *Client) ReceiveFilesFrom(done chan bool, ch <-chan File) {
	bufw := bufio.NewWriter(c.RWC)
	for {
		select {
		case file := <-ch:
			if _, err := bufw.WriteString(fmt.Sprintf("send -- | Received file: %s\n", file.Filename)); err != nil {
				log.Printf("An error occured writing: %s\n", err.Error())
			}

			if err := bufw.Flush(); err != nil {
				log.Printf("An error occured flushing: %s\n", err.Error())
			}

			c.Files = append(c.Files, file)
		case <-done:
			return
		}
	}
}

func (c *Client) ListFiles() {
	bufw := bufio.NewWriter(c.RWC)
	_, err := bufw.WriteString(fmt.Sprintf("list -- | Remaining Bandwidth: %d KB\n", c.Bandwidth))
	if err != nil {
		log.Printf("Error occured while writing: %s", err.Error())
	}
	if err := bufw.Flush(); err != nil {
		log.Printf("An error occured flushing: %s\n", err.Error())
	}

	w := new(tabwriter.Writer)
	w.Init(c.RWC, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "list -- |\tName\tSize\tSecrecy Value")
	for _, file := range c.Files {
		fmt.Fprintf(w, "list -- |\t%s\t%d\t%d\n", file.Filename, file.Size, file.Secrecy)
	}
	w.Flush()
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
