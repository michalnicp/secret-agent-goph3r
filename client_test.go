package main

import (
	"bytes"
	"testing"
)

// Dummy net Conn that is a ReadWriteCloser
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

func TestFileHandler(t *testing.T) {
	rwc := NewCloseableBuffer()
	client := NewClient(rwc, "gopher1")

	done := make(chan bool)

	go client.FileHandler(done)

	file := File{
		Filename: "testfile.txt",
		Size:     100,
		Secrecy:  100,
	}
	client.Filechan <- file

	for _, f := range client.Files {
		if f.Filename == file.Filename {
			return
		}
	}
	t.Errorf("Expected file %#v in %#v, found none", file, client.Files)
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
