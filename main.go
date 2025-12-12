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
	conn net.Conn
	id   string
}

func newChatConn(conn net.Conn) *chatConn {
	return &chatConn{
		conn: conn,
		id:   uuid.New().String(),
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
	}

}

func handleConnection(chatC *chatConn) {
	defer deleteConnection(chatC)
	scanner := bufio.NewScanner(chatC.conn)

	for scanner.Scan() {
		readStr := scanner.Text() + "\n" // add newline back for display

		// broadcast readStr to others (your existing logic)
		broadcast(chatC, readStr)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("read error:", err)
	}

}
func broadcast(sender *chatConn, message string) {
	var toDelete []*chatConn
	connsMu.RLock()
	for _, c := range conns {
		if c != sender {
			_, writeErr := c.conn.Write([]byte(message))
			if writeErr != nil {
				fmt.Println("Error writing to connection:", writeErr.Error())
				toDelete = append(toDelete, c) // mark only
			}
		}
	}
	connsMu.RUnlock()
	// Now delete outside the read lock
	for _, c := range toDelete {
		deleteConnection(c)
	}

}

func deleteConnection(chatC *chatConn) {
	fmt.Printf("closing connection: %v\n", chatC.id)
	err := chatC.conn.Close()
	if err != nil {
		fmt.Printf("error while closing connection: %v\n", err.Error())
	}
	connsMu.Lock()
	delete(conns, chatC.id)
	connsMu.Unlock()
}
