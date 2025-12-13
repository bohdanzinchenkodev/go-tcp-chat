package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var (
	conns      = make(map[string]*chatConn)
	connsMu    sync.RWMutex
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,20}$`)
)

type chatConn struct {
	conn      net.Conn
	id        string
	send      chan string
	closeOnce sync.Once
	username  string
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

	//collect username
	systemMessage(chatC, "Please type your username:\n")

	for scanner.Scan() {
		readStr := scanner.Text()

		//if username is empty set it
		if chatC.username == "" {
			//validate username
			readStr = strings.TrimSpace(readStr)
			usernameErr := validateUsername(readStr)
			if usernameErr != "" {
				systemMessage(chatC, usernameErr)
				continue
			}
			//set username and send hello message
			chatC.username = readStr
			systemMessage(chatC, "Welcome To Chat, "+chatC.username+"!\n")

			//notify chat users about new chatter
			readStr = formatSystemMessage(readStr) + " joining the chat\n"
		} else {
			readStr = "[" + chatC.username + "]" + readStr + "\n"
		}
		broadcast(chatC, readStr)

	}
	if err := scanner.Err(); err != nil {
		fmt.Println("read error:", err)
	}
}
func systemMessage(chatC *chatConn, message string) {
	message = formatSystemMessage(message)
	sendMessage(chatC, message)
}
func formatSystemMessage(message string) string {
	return "[system] " + message
}
func sendMessage(chatC *chatConn, message string) {
	select {
	case chatC.send <- message:

	default:
		deleteConnection(chatC)
	}

}
func validateUsername(username string) string {
	if username == "" {
		return "username cannot be empty\n"
	} else if !usernameRe.MatchString(username) {
		return "Invalid username. Use letters, numbers, _ or -, max 20 chars.\n"
	}
	return ""
}
func broadcast(sender *chatConn, message string) {
	connsMu.RLock()
	targets := make([]*chatConn, 0, len(conns))
	for _, c := range conns {
		if c != sender && c.username != "" {
			targets = append(targets, c)
		}
	}

	connsMu.RUnlock()
	for _, c := range targets {
		sendMessage(c, message)
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
		if chatC.username != "" {
			broadcast(chatC, formatSystemMessage(chatC.username+" leaving the chat\n"))
		}
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
