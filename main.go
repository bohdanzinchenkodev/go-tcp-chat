package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/google/uuid"
)

var (
	conns   = make(map[string]*chatConn)
	connsMu sync.RWMutex
)

type chatConn struct {
	conn      net.Conn
	id        string
	send      chan string
	closeOnce sync.Once
}

func newChatConn(conn net.Conn) *chatConn {
	return &chatConn{
		conn: conn,
		id:   uuid.New().String(),
		send: make(chan string, 16),
	}
}
func main() {
	args := os.Args

	if len(args) <= 1 {
		fmt.Print("Please provide a port number as an argument.\n")
		os.Exit(0)
	}

	port := args[1]
	ln, listenErr := net.Listen("tcp", ":"+port)
	if listenErr != nil {
		fmt.Printf("Error starting server: %v\n", listenErr)
		os.Exit(0)
	}

	for {
		fmt.Println("Waiting for connection...")

		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			fmt.Println("Error accepting connection:", acceptErr.Error())
			continue
		}

		connsMu.Lock()
		chatC := newChatConn(conn)
		conns[chatC.id] = chatC
		connsMu.Unlock()

		fmt.Printf("Connection accepted: %v\n", chatC.id)

		go handleConnection(chatC)
		go writePump(chatC)
	}

}

func handleConnection(chatC *chatConn) {
	defer deleteConnection(chatC)
	scanner := bufio.NewScanner(chatC.conn)

	for scanner.Scan() {
		readStr := scanner.Text() + "\n"

		// broadcast readStr to others
		broadcast(chatC, readStr)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("read error:", err)
	}

}
func broadcast(sender *chatConn, message string) {
	connsMu.RLock()
	targets := make([]*chatConn, 0, len(conns))
	for _, c := range conns {
		if c != sender {
			targets = append(targets, c)
		}
	}

	connsMu.RUnlock()
	for _, c := range targets {
		select {
		case c.send <- message:

		default:
			deleteConnection(c)
		}
	}
}
func writePump(chatC *chatConn) {
	for message := range chatC.send {
		_, writeErr := chatC.conn.Write([]byte(message))
		if writeErr != nil {
			fmt.Println("Error writing to connection:", writeErr.Error())
			deleteConnection(chatC)
			return
		}
	}
}

func deleteConnection(chatC *chatConn) {
	chatC.closeOnce.Do(func() {
		fmt.Printf("closing connection: %v\n", chatC.id)
		//close channel
		close(chatC.send)
		//close connection
		err := chatC.conn.Close()
		if err != nil {
			fmt.Printf("error while closing connection: %v\n", err.Error())
		}
		//delete connection from map
		connsMu.Lock()
		delete(conns, chatC.id)
		connsMu.Unlock()
	})
}
