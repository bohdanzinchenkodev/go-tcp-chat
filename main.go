package main

import (
	"bufio"
	"fmt"
	"go-tcp-chat/hub"
	"net"
	"os"
)

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
	hub.AddRoom("room1")
	hub.AddRoom("room2")
	hub.AddRoom("room3")
	go hub.StartEngine()

	for {
		fmt.Println("Waiting for connection...\n")

		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			fmt.Println("Error accepting connection:", acceptErr.Error())
			continue
		}

		chatC := hub.NewChatConn(conn)
		connectEvent(chatC)

		fmt.Printf("Connection accepted: %v\n", chatC.Id)

		go readPump(chatC)
		go writePump(chatC)
		/*go eventFromRead(chatC)*/
	}
}
func connectEvent(chatC *hub.ChatConn) {
	ce := hub.ChatEvent{
		ChatC:     chatC,
		EventType: hub.ConnectE,
	}
	hub.EventChan <- ce
}
func disconnectEvent(chatC *hub.ChatConn) {
	ce := hub.ChatEvent{
		ChatC:     chatC,
		EventType: hub.DisconnectE,
	}
	hub.EventChan <- ce
}

func readPump(chatC *hub.ChatConn) {
	scanner := bufio.NewScanner(chatC.Conn)
	for scanner.Scan() {
		select {
		case hub.EventChan <- hub.ChatEvent{
			ChatC:     chatC,
			EventType: hub.RawInputE,
			Input:     scanner.Text(),
		}:
		default:
			disconnect(chatC)
			return
		}
	}
	disconnect(chatC)
}
func writePump(chatC *hub.ChatConn) {
	for message := range chatC.Write {
		_, writeErr := chatC.Conn.Write([]byte(message))
		if writeErr != nil {
			fmt.Println("Error writing to connection:", writeErr.Error())
			disconnect(chatC)
			return
		}
	}
}
func disconnect(chatC *hub.ChatConn) {
	chatC.Conn.Close()
	close(chatC.Write)
	disconnectEvent(chatC)
}
