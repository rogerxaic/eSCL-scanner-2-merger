package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

type WebSocketManager struct {
	clients    map[*websocket.Conn]bool
	clientsMux sync.Mutex
	upgrader   websocket.Upgrader
}

// Implementation details...
