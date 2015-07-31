package main

import (
	"bytes"
	"testing"
)

type CloseableBuffer struct {
	*bytes.Buffer
}

func (*CloseableBuffer) Close() error {
	return nil
}

func NewCloseableBuffer() *CloseableBuffer {
	return &CloseableBuffer{bytes.NewBuffer(nil)}
}

func TestNewClient(t *testing.T) {
	rwc := NewCloseableBuffer()
	nickname := "gopher1"

	client := NewClient(rwc, nickname)
	expectedClient := &Client{
		RWC:      rwc,
		Nickname: nickname,
	}
	if client.RWC != expectedClient.RWC {
		t.Fatalf("Error creating client ReadWriteCloser: expected %v, got %v", client.RWC, expectedClient.RWC)
	}
	if client.Nickname != expectedClient.Nickname {
		t.Fatalf("Error creating client nickname: expected %s, got %s", client.Nickname, expectedClient.Nickname)
	}
}

func TestInitClient(t *testing.T) {
	rwc := NewCloseableBuffer()

	clientch := make(chan *Client)
	done := make(chan bool)
	go func() {
		InitClient(rwc, clientch)
		done <- true
	}()

	_, err := rwc.WriteString("gopher1\n")
	if err != nil {
		t.Fatalf("An error occured while writing: %s", err.Error())
	}

	client := <-clientch
	expectedClient := NewClient(rwc, "gopher1")
	if client.RWC != expectedClient.RWC {
		t.Fatalf("Error creating client ReadWriteCloser: expected %v, got %v", client.RWC, expectedClient.RWC)
	}
	if client.Nickname != expectedClient.Nickname {
		t.Fatalf("Error creating client nickname: expected %s, got %s", client.Nickname, expectedClient.Nickname)
	}
}

func TestReadLinesInto(t *testing.T) {
	rwc := NewCloseableBuffer()
	client := NewClient(rwc, "gopher1")

	msgchan := make(chan Message)
	done := make(chan bool)

	go client.ReadLinesInto(done, msgchan)

	_, err := rwc.WriteString("test message\n")
	if err != nil {
		t.Fatalf("An error occured while writing: %s", err.Error())
	}

	msg := <-msgchan
	expectedMsg := Message{
		From: client,
		Text: "test message\n",
	}
	if msg != expectedMsg {
		t.Errorf("Expected message \"%s\" from %s, got \"%s\" from %s", expectedMsg.Text, expectedMsg.From.Nickname, msg.Text, msg.From.Nickname)
	}
}

func TestWriteLinesFrom(t *testing.T) {
	rwc := NewCloseableBuffer()
	client := NewClient(rwc, "gopher1")

	msgchan := make(chan Message)
	done := make(chan bool)

	go client.WriteLinesFrom(done, msgchan)

	msg := Message{
		From: client,
		Text: "test message\n",
	}
	msgchan <- msg

	line, err := rwc.ReadString('\n')
	if err != nil {
		t.Errorf("Error reading: %s", err.Error())
	}
	expectedMsg := Message{
		From: client,
		Text: line,
	}
	if msg != expectedMsg {
		t.Errorf("Expected message \"%s\" from %s, got \"%s\" from %s", expectedMsg.Text, expectedMsg.From.Nickname, msg.Text, msg.From.Nickname)
	}
}

func TestHasFile(t *testing.T) {
	rwc := NewCloseableBuffer()
	client := NewClient(rwc, "gopher1")

	file := File{
		Filename: "testfile.txt",
		Size:     100,
		Secrecy:  100,
	}

	client.Files = []File{file}
	if !client.HasFile("testfile.txt") {
		t.Errorf("Expected file %#v, client has %#v", file, client.Files)
	}
}

func TestReceiveFilesFrom(t *testing.T) {
	rwc := NewCloseableBuffer()
	client := NewClient(rwc, "gopher1")

	fileChan := make(chan File)
	done := make(chan bool)

	go client.ReceiveFilesFrom(done, fileChan)

	file := File{
		Filename: "testfile.txt",
		Size:     100,
		Secrecy:  100,
	}
	fileChan <- file

	if !client.HasFile("testfile.txt") {
		t.Errorf("Expected file %#v, client has %#v", file, client.Files)
	}
}

func TestListFiles(t *testing.T) {
	rwc := NewCloseableBuffer()
	client := NewClient(rwc, "gopher1")

	file := File{
		Filename: "testfile.txt",
		Size:     100,
		Secrecy:  100,
	}

	client.Files = []File{file}

	client.ListFiles()

	list := ""
	line, err := rwc.ReadString('\n')
	if err != nil {
		t.Errorf("Error while reading: %s", err.Error())
	}
	list += line

	line, err = rwc.ReadString('\n')
	if err != nil {
		t.Errorf("Error while reading: %s", err.Error())
	}
	list += line

	line, err = rwc.ReadString('\n')
	if err != nil {
		t.Errorf("Error while reading: %s", err.Error())
	}
	list += line

	expectedList := "list -- | Remaining Bandwidth: 100 KB\n" +
		"list -- |  Name          Size  Secrecy Value\n" +
		"list -- |  testfile.txt  100   100\n"

	if list != expectedList {
		t.Errorf("Expected:\n%s got:\n%s", expectedList, list)
	}
}

func TestSendFileTo(t *testing.T) {
	rwc := NewCloseableBuffer()
	client := NewClient(rwc, "gopher1")

	fileChan := make(chan File, 1)

	file := File{
		Filename: "testfile.txt",
		Size:     1,
		Secrecy:  100,
	}

	client.Files = []File{file}

	client.SendFileTo("testfile.txt", fileChan, true)
	if client.HasFile("testfile.txt") {
		t.Errorf("Expected file %#v, client has %#v", file, client.Files)
	}
	if client.Bandwidth != 99 {
		t.Errorf("Expected bandwidth %d, got %d", 99, client.Bandwidth)
	}

	receivedFile := <-fileChan
	if receivedFile.Filename != "testfile.txt" {
		t.Errorf("Exected received file %#v, got %#v", file, receivedFile)
	}
}
