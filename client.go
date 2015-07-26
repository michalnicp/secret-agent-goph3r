package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"text/tabwriter"
)

type Client struct {
	Conn      net.Conn
	Nickname  string
	Ch        chan Message
	Files     []File
	Filechan  chan File
	Bandwidth int
	Done      bool
}

func (c Client) ReadLinesInto(ch chan<- Message) {
	bufc := bufio.NewReader(c.Conn)
	for {
		line, err := bufc.ReadString('\n')
		if err != nil {
			break
		}
		message := Message{
			From: c,
			Text: line,
		}
		ch <- message
	}
}

func (c Client) WriteLinesFrom(ch <-chan Message) {
	for msg := range ch {
		_, err := io.WriteString(c.Conn, msg.Text)
		if err != nil {
			return
		}
	}
}

func (c Client) ListFiles() {
	io.WriteString(c.Conn, fmt.Sprintf("list -- | Remaining Bandwidth: %d KB\n", c.Bandwidth))
	w := new(tabwriter.Writer)
	w.Init(c.Conn, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "list -- |\tName\tSize\tSecrecy Value")
	for _, file := range c.Files {
		fmt.Fprintf(w, "list -- |\t%s\t%d\t%d\n", file.Filename, file.Size, file.Secrecy)
	}
	w.Flush()
}
