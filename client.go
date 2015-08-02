package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"sync"
)

type Client struct {
	RWC              io.ReadWriteCloser
	Nickname         string
	MsgChan          chan Message
	FileChan         chan File
	InputChan        chan string
	Files            []File
	Bandwidth        int
	DoneSendingFiles bool
	Game             *Game
	Done             chan bool
}

func NewClient(rwc io.ReadWriteCloser, nickname string) *Client {
	return &Client{
		RWC:              rwc,
		Nickname:         nickname,
		MsgChan:          make(chan Message, 10), // TODO do I need a buff chan
		FileChan:         make(chan File),
		InputChan:        make(chan string),
		Files:            make([]File, 0),
		Bandwidth:        100,
		DoneSendingFiles: false,
		Done:             make(chan bool),
		Game:             &Game{},
	}
}

func GetNickname(rw io.ReadWriter) (string, error) {
	bufw := bufio.NewWriter(rw)
	for {
		nickname, err := Prompt(rw, NICK_MSG)
		if err != nil {
			return "", err
		}

		nickname = strings.TrimSpace(nickname)
		if nickname == "" || nickname == "Glenda" {
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
		return nickname, nil
	}
}

func (c *Client) Start() {
	var wg sync.WaitGroup

	wg.Add(2)
	go c.MsgHandler(&wg)
	go c.FileHandler(&wg)

	// InputHandler is closed by closing the RWC connection
	go c.InputHandler()

	wg.Wait()
	c.RWC.Close()
}

func (c *Client) End() {
	close(c.MsgChan)
	close(c.FileChan)
	close(c.InputChan)
}

func (c *Client) InputHandler() {
	bufr := bufio.NewReader(c.RWC)

	// Go routine to handle input, non blocking
	// TODO rewrite to use net.conn timeout feature
	// TODO ignore input before starting client
	go func() {
		for {
			line, err := bufr.ReadString('\n')
			log.Printf("Input from %s: %s", c.Nickname, line)
			if err != nil {
				log.Printf("An error occured while reading: %s\n", err.Error())
				close(c.InputChan)
				return
			}
			c.InputChan <- line
		}
	}()

	for input := range c.InputChan {
		c.ParseInput(input)
	}
}

func (c *Client) ParseInput(input string) {
	re := regexp.MustCompile(`(\/\w+) *(\w*) *(.*)`)
	reResult := re.FindStringSubmatch(input)
	if reResult == nil {
		c.MsgChan <- Message{
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
		c.MsgChan <- Message{
			Text: "err -- | Invalid command, try /help to see valid commands\n",
		}
	}
}

func (c *Client) FileHandler(wg *sync.WaitGroup) {
	bufw := bufio.NewWriter(c.RWC)
	for f := range c.FileChan {
		if _, err := bufw.WriteString(fmt.Sprintf("send -- | Received file: %s\n", f.Filename)); err != nil {
			log.Printf("An error occured writing: %s\n", err.Error())
		}

		if err := bufw.Flush(); err != nil {
			log.Printf("An error occured flushing: %s\n", err.Error())
		}

		c.Files = append(c.Files, f)
	}
	wg.Done()
}

func (c *Client) MsgHandler(wg *sync.WaitGroup) {
	bufw := bufio.NewWriter(c.RWC)
	for msg := range c.MsgChan {
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
	wg.Done()
}

func (c *Client) Help() {
	c.MsgChan <- Message{Text: HELP_MSG}
}

func (c *Client) SendMsgTo(to string, text string) {
	if to == "Glenda" {
		if text == "done" {
			c.DoneSendingFiles = true
			c.Game.WG.Done()
			return
		} else {
			c.MsgChan <- Message{Text: GLENDA_MSG}
		}
		return
	}
	for _, client := range c.Game.Clients {
		if to == client.Nickname {
			client.MsgChan <- Message{
				From: c.Nickname,
				To:   to,
				Text: fmt.Sprintf("%s | %s\n", c.Nickname, text),
			}
			return
		}
	}
	c.MsgChan <- Message{Text: fmt.Sprintf("err -- | %s does not exist\n", to)}
}

func (c *Client) ListFiles() {
	bufw := bufio.NewWriter(c.RWC)
	_, err := bufw.WriteString(fmt.Sprintf("list -- | Remaining Bandwidth: %d KB\n", c.Bandwidth))
	if err != nil {
		log.Printf("Error occured while writing: %s", err.Error())
	}

	_, err = bufw.WriteString(fmt.Sprintf("list -- | %20s  %8s  %13s\n", "Filename", "Size", "Secrecy Value"))
	for _, f := range c.Files {
		_, err = bufw.WriteString(fmt.Sprintf("list -- | %20s  %5d KB  %13d\n", f.Filename, f.Size, f.Secrecy))
		if err != nil {
			log.Printf("Error occured while writing: %s", err.Error())
		}
	}

	if err := bufw.Flush(); err != nil {
		log.Printf("An error occured flushing: %s\n", err.Error())
	}
}

func (c *Client) SendFileTo(to string, filename string) {
	if c.DoneSendingFiles {
		return
	}
	// TODO rewrite better function
	foundFile := false
	foundClient := false

	var i int
	for j, file := range c.Files {
		if file.Filename == filename {
			foundFile = true
			i = j
			if to == "Glenda" {
				foundClient = true
				c.Game.FileChan <- file
				// Use up bandwidth when sending to Glenda
				c.Bandwidth -= file.Size
				if c.Bandwidth < 0 {
					// fail the game
					c.Game.Status = FAIL
					c.Game.End()
				}
				c.MsgChan <- Message{
					Text: fmt.Sprintf("send -- | Sent file: %s", file.Filename),
				}
			}
			for _, client := range c.Game.Clients {
				if to == client.Nickname {
					foundClient = true
					client.FileChan <- file
					break
				}
			}
			break
		}
	}

	if !foundFile {
		c.MsgChan <- Message{
			Text: fmt.Sprintf("err -- | Error sending file: file \"%s\" does not exist", filename),
		}
		return
	}
	if !foundClient {
		c.MsgChan <- Message{
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
		lookText += ("look -- | " + client.Nickname + "\n")
	}
	lookText += "look -- | Glenda\n"
	c.MsgChan <- Message{Text: lookText}
}
