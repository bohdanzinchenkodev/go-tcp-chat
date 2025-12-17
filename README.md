# Go TCP Chat

Tiny TCP chat server written in Go, for learning purposes. The server accepts plain TCP connections and lets multiple clients chat in simple rooms.

## Prerequisites

- Go 1.25+

## Run

```bash
go run main.go 8080
```

This starts the server on `localhost:8080`.

## Connect

In another terminal:

```bash
nc localhost 8080
```

Open more terminals with the same command to simulate multiple users.

First line you type is your **username**. Then you can chat and use a few simple commands:

- `/rooms` – list rooms
- `/join room1` – join a room
- `/leave` – leave current room
- `/help` – list commands

That’s it—just a small playground to explore TCP, goroutines, and channels.
