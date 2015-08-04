package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	RWC              io.ReadWriteCloser
	Name             string
	MsgCh            chan Message
	FileCh           chan File
	InputCh          chan string
	Files            []File
	DoneSendingFiles bool
	Bandwidth        int
	Game             *Game
	Done             chan bool
}

func NewClient(rwc io.ReadWriteCloser) *Client {
	return &Client{
		RWC:     rwc,
		MsgCh:   make(chan Message, 10), // TODO do I need a buff chan
		FileCh:  make(chan File),
		InputCh: make(chan string),
		Files:   make([]File, 0),
		Done:    make(chan bool),
	}
}

func (c *Client) GetName() (string, error) {
	bufw := bufio.NewWriter(c.RWC)
	for {
		name, err := c.Prompt(NICK_MSG)
		if err != nil {
			return "", err
		}

		if name == "" || name == "Glenda" {
			if _, err := bufw.WriteString("Invalid Username\n"); err != nil {
				log.Printf("Error occuring while writing: %s\n", err.Error())
				return "", err
			}
			if err := bufw.Flush(); err != nil {
				log.Printf("Error occured while flushing: %s\n", err.Error())
				return "", err
			}
			continue
		}
		return name, nil
	}
}

func (c *Client) Start() {
	go c.MsgHandler()
	go c.FileHandler()
	go c.InputHandler()
}

func (c *Client) End() {
	//TODO if client has already left via ctrl-C, this panics
	//TODO flush channels before closing
	time.Sleep(1)
	if c.Game != nil {
		c.Game.RmCh <- c
	}
	//close(c.MsgCh)
	//close(c.FileCh)
	//close(c.InputCh)
	c.RWC.Close()
	log.Printf("closed client %s", c.Name)
}

func (c *Client) InputHandler() {
	bufr := bufio.NewReader(c.RWC)

	// Go routine to handle input, non blocking
	// TODO rewrite to use net.conn timeout feature
	go func() {
		for {
			line, err := bufr.ReadString('\n')
			if err != nil {
				log.Printf("An error occured while reading: %s\n", err.Error())
				c.End()
				return
			}
			c.InputCh <- line
		}
	}()

	for input := range c.InputCh {
		c.ParseInput(input)
	}
}

func (c *Client) ParseInput(input string) {
	// Toss input if game is in lobby status
	if c.Game.Status == LOBBY {
		return
	}

	re := regexp.MustCompile(`(\/\w+) *(\S*) *(.*)`)
	reResult := re.FindStringSubmatch(input)
	if reResult == nil {
		c.MsgCh <- Message{
			Text: "err -- | Invalid command, try /help to see valid commands\n",
		}
		return
	}
	command := reResult[1]
	arg1 := reResult[2]
	arg2 := reResult[3]
	switch command {
	case "/help":
		c.Help()
	case "/msg":
		c.SendMsgTo(arg1, arg2)
	case "/list":
		c.ListFiles()
	case "/send":
		c.SendFileTo(arg1, arg2)
	case "/look":
		c.Look()
	default:
		c.MsgCh <- Message{
			Text: "err -- | Invalid command, try /help to see valid commands\n",
		}
	}
}

func (c *Client) FileHandler() {
	bufw := bufio.NewWriter(c.RWC)
	for f := range c.FileCh {
		if _, err := bufw.WriteString(fmt.Sprintf("send -- | Received file: %s\n", f.Filename)); err != nil {
			log.Printf("An error occured writing: %s\n", err.Error())
			c.End()
			return
		}
		if err := bufw.Flush(); err != nil {
			log.Printf("An error occured flushing: %s\n", err.Error())
			c.End()
			return
		}

		c.Files = append(c.Files, f)
	}
}

func (c *Client) MsgHandler() {
	bufw := bufio.NewWriter(c.RWC)
	for msg := range c.MsgCh {
		if _, err := bufw.WriteString(msg.Text); err != nil {
			log.Printf("An error occured writing: %s\n", err.Error())
			c.End()
			return
		}
		if err := bufw.Flush(); err != nil {
			log.Printf("An error occured flushing: %s\n", err.Error())
			c.End()
			return
		}
	}
}

func (c *Client) Help() {
	c.MsgCh <- Message{Text: HELP_MSG}
}

func (c *Client) SendMsgTo(to string, text string) {
	c.Game.MsgCh <- Message{
		From: c.Name,
		To:   to,
		Text: text,
	}
}

func (c *Client) ListFiles() {
	bufw := bufio.NewWriter(c.RWC)
	_, err := bufw.WriteString(fmt.Sprintf("list -- | Remaining Bandwidth: %d KB\n", c.Bandwidth))
	if err != nil {
		log.Printf("Error occured while writing: %s", err.Error())
		c.End()
		return
	}

	_, err = bufw.WriteString(fmt.Sprintf("list -- | %20s  %8s  %13s\n", "Filename", "Size", "Secrecy Value"))
	for _, f := range c.Files {
		_, err = bufw.WriteString(fmt.Sprintf("list -- | %20s  %5d KB  %13d\n", f.Filename, f.Size, f.Secrecy))
		if err != nil {
			log.Printf("Error occured while writing: %s", err.Error())
			c.End()
			return
		}
	}

	if err := bufw.Flush(); err != nil {
		log.Printf("An error occured flushing: %s\n", err.Error())
		c.End()
		return
	}
}

func (c *Client) SendFileTo(to string, filename string) {
	// TODO rewrite to instead route file through server
	foundFile := false
	foundClient := false

	var i int
	for j, file := range c.Files {
		if file.Filename == filename {
			foundFile = true
			i = j
			if to == "Glenda" {
				foundClient = true
				// Use up bandwidth when sending to Glenda
				c.Game.FileCh <- file
				c.Bandwidth -= file.Size
				if c.Bandwidth < 0 {
					// fail the game
					c.Game.Status = FAIL
					return
				}
				c.MsgCh <- Message{
					Text: fmt.Sprintf("send -- | Sent file: %s\n", file.Filename),
				}
			}
			for _, client := range c.Game.Clients {
				if to == client.Name {
					foundClient = true
					client.FileCh <- file
					break
				}
			}
			break
		}
	}

	if !foundFile {
		c.MsgCh <- Message{
			Text: fmt.Sprintf("err -- | Error sending file: file \"%s\" does not exist\n", filename),
		}
		return
	}
	if !foundClient {
		c.MsgCh <- Message{
			Text: fmt.Sprintf("err -- | Error sending file: client \"%s\" does not exist", to),
		}
		return
	}

	files := c.Files
	newFiles := make([]File, 0, len(files)-1)
	if i == 0 {
		newFiles = files[1:]
	} else if i == len(files)-1 {
		newFiles = files[:len(files)-1]
	} else {
		newFiles = append(files[:i-1], files[i:]...)
	}
	c.Files = newFiles
}

func (c *Client) Look() {
	lookText := "look -- | You look around at your co-workers' nametages:\n"
	for _, client := range c.Game.Clients {
		lookText += ("look -- | " + client.Name + "\n")
	}
	lookText += "look -- | Glenda\n"
	c.MsgCh <- Message{Text: lookText}
}

func (c *Client) WriteString(text string) error {
	bufw := bufio.NewWriter(c.RWC)
	if _, err := bufw.WriteString(text); err != nil {
		log.Printf("Error occured while writing: %s", err.Error())
		return err
	}

	if err := bufw.Flush(); err != nil {
		log.Printf("An error occured flushing: %s\n", err.Error())
		return err
	}
	return nil
}

func (c *Client) ReadLine() (string, error) {
	bufr := bufio.NewReader(c.RWC)
	line, err := bufr.ReadString('\n')
	if err != nil {
		log.Printf("An error occured reading: %s\n", err.Error())
		return "", err
	}
	line = strings.TrimSpace(line)

	return line, nil
}

func (c *Client) Prompt(question string) (string, error) {
	if err := c.WriteString(question); err != nil {
		return "", err
	}

	ans, err := c.ReadLine()
	if err != nil {
		return "", err
	}
	return ans, nil
}
