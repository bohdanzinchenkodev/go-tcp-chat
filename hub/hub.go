package hub

import (
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type ChatConn struct {
	Conn        net.Conn
	Id          string
	Write       chan string
	Username    string
	CurrentRoom string
	CloseOnce   sync.Once
}
type ChatEvent struct {
	ChatC     *ChatConn
	EventType string
	Input     string
}

func NewChatConn(conn net.Conn) *ChatConn {
	return &ChatConn{
		Conn:  conn,
		Id:    uuid.New().String(),
		Write: make(chan string, 16),
	}
}

type room struct {
	name  string
	users map[string]struct{}
}

const (
	ConnectE    = "connect"
	RawInputE   = "rawInput"
	DisconnectE = "disconnect"
)
const (
	CmdRooms = "rooms"
	CmdJoin  = "join"
	CmdLeave = "leave"
	CmdHelp  = "help"
)

var (
	EventChan  chan ChatEvent = make(chan ChatEvent, 32)
	usernameRe                = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,20}$`)
	conns                     = make(map[string]*ChatConn)
	rooms                     = make(map[string]*room)
)

func AddRoom(name string) {
	r := room{
		name:  name,
		users: make(map[string]struct{}),
	}
	rooms[name] = &r
}
func StartEngine() {
	for e := range EventChan {
		switch e.EventType {
		case ConnectE:
			connect(&e)
		case RawInputE:
			processRawInput(&e)
		/*case SetUsernameE:
			setUsername(&e)
		case CmdE:
			processCmd(&e)
		case ChatMessageE:
			chatMessage(&e)*/
		case DisconnectE:
			disconnect(&e)
		default:

		}
	}
}

func connect(e *ChatEvent) {
	conns[e.ChatC.Id] = e.ChatC
	writeToConn(e.ChatC, "Welcome to the chat server! Please set your username\n")
}
func disconnect(e *ChatEvent) {
	delete(conns, e.ChatC.Id)

	if roomName := e.ChatC.CurrentRoom; roomName != "" {
		if r, ok := rooms[roomName]; ok {
			broadcast(e.ChatC, formatSystemMessage(e.ChatC.Username+" just left the room.\n"))
			delete(r.users, e.ChatC.Id)
		}
		e.ChatC.CurrentRoom = ""
	}

	_ = e.ChatC.Conn.Close()
	close(e.ChatC.Write)

}
func processRawInput(e *ChatEvent) {
	chatC := e.ChatC
	if chatC.Username == "" {
		setUsername(e)
	} else if strings.HasPrefix(e.Input, "/") {
		processCmd(e)
	} else {
		chatMessage(e)
	}
}
func setUsername(e *ChatEvent) {
	if e.Input == "" {
		writeToConn(e.ChatC, "Username cannot be empty. Please try again:\n")
		return
	}
	proposedUsername := strings.TrimSpace(e.Input)
	if !usernameRe.MatchString(proposedUsername) {
		writeToConn(e.ChatC, "Invalid username. Usernames can only contain letters, numbers, underscores, and hyphens, and must be 1-20 characters long. Please try again:\n")
		return
	}
	e.ChatC.Username = proposedUsername
	writeToConn(e.ChatC, "Username set to "+proposedUsername+"\n")
	writeToConn(e.ChatC, "You can learn list of commands by typing /help \n")
}
func processCmd(e *ChatEvent) {
	args := inputToArgs(e.Input)
	if len(args) < 1 {
		return
	}
	cmd := args[0]
	switch cmd {
	case CmdRooms:
		roomNames := make([]string, 0, len(rooms))
		for _, r := range rooms {
			roomNames = append(roomNames, r.name)
		}

		roomNamesStr := strings.Join(roomNames, "\n")
		writeToConn(e.ChatC, roomNamesStr+"\n")
	case CmdJoin:
		if e.ChatC.CurrentRoom != "" {
			writeToConn(e.ChatC, "You are already in a room. Please leave it before joining another one.\n")
			return
		}
		if len(args) < 2 {
			writeToConn(e.ChatC, "Please provide a room name to join.\n")
			return
		}

		roomName := args[1]
		r, exists := rooms[roomName]
		if !exists {
			writeToConn(e.ChatC, "Room does not exist.\n")
			return
		}
		_, uExists := r.users[e.ChatC.Id]
		if uExists {
			writeToConn(e.ChatC, "You are already in the room\n")
			return
		}
		r.users[e.ChatC.Id] = struct{}{}
		e.ChatC.CurrentRoom = roomName

		writeToConn(e.ChatC, "You just joined: "+roomName+"\n")
		broadcast(e.ChatC, formatSystemMessage(e.ChatC.Username+" just joined the room.\n"))
	case CmdLeave:
		roomName := e.ChatC.CurrentRoom
		if roomName == "" {
			writeToConn(e.ChatC, "You are not in any room.\n")
			return
		}

		r, exists := rooms[roomName]
		if !exists {
			writeToConn(e.ChatC, "Room does not exist.\n")
			return
		}

		delete(r.users, e.ChatC.Id)
		e.ChatC.CurrentRoom = ""

		writeToConn(e.ChatC, "You just left: "+roomName+"\n")
		broadcast(e.ChatC, formatSystemMessage(e.ChatC.Username+" just left the room.\n"))
	case CmdHelp:
		writeToConn(e.ChatC, "/rooms - list of rooms\n")
		writeToConn(e.ChatC, "/join [roomname] - join room \n")
		writeToConn(e.ChatC, "/leave - leave current room \n")
	default:
	}
}
func chatMessage(e *ChatEvent) {
	if e.ChatC.CurrentRoom == "" {
		writeToConn(e.ChatC, "Nobody can hear you.\n")
		return
	}
	message := strings.TrimSpace(e.Input)
	if message == "" {
		return
	}

	broadcast(e.ChatC, formatChatMessage(e.ChatC.Username, message))
}
func formatChatMessage(username, message string) string {
	return "[" + username + "]: " + message + "\n"
}
func formatSystemMessage(message string) string {
	return "[system]: " + message
}
func inputToArgs(input string) []string {
	trimmedInput := strings.TrimPrefix(input, "/")
	parts := strings.Fields(trimmedInput)
	return parts
}
func broadcast(chatC *ChatConn, message string) {
	roomName := chatC.CurrentRoom
	if roomName == "" {
		return
	}
	r, exists := rooms[roomName]
	if !exists {
		return
	}
	for userId := range r.users {
		if userId == chatC.Id {
			continue
		}
		recipientConn, connExists := conns[userId]
		if connExists {
			select {
			case recipientConn.Write <- message:
			default:
				//drop message
			}
		}
	}

}
func writeToConn(chatC *ChatConn, msg string) {
	chatC.Write <- formatSystemMessage(msg)
}
