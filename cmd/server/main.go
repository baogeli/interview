package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Broadcast struct {
	mu         sync.Mutex
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

var broadcast *Broadcast

func NewBroadcast() *Broadcast {
	return &Broadcast{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (b *Broadcast) Run() {
	for {
		select {
		case client := <-b.register:
			b.mu.Lock()
			b.clients[client] = true
			b.mu.Unlock()
			log.Printf("client %s joined (total: %d)", client.conn.RemoteAddr(), len(b.clients))

		case client := <-b.unregister:
			b.mu.Lock()
			if _, ok := b.clients[client]; ok {
				delete(b.clients, client)
				close(client.send)
				log.Printf("client %s left (total: %d)", client.conn.RemoteAddr(), len(b.clients))
			}
			b.mu.Unlock()

		case message := <-b.broadcast:
			b.mu.Lock()
			for client := range b.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(b.clients, client)
				}
			}
			b.mu.Unlock()
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func serverHandler(ws *websocket.Conn) {
	client := &Client{conn: ws, send: make(chan []byte, 256)}
	broadcast.register <- client

	go func() {
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("read error: %v", err)
				}
				broadcast.unregister <- client
				return
			}
			log.Printf("received from %s: %s", ws.RemoteAddr(), string(msg))
			broadcast.broadcast <- msg
		}
	}()

	for {
		msg, ok := <-client.send
		if !ok {
			ws.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
		ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := ws.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	broadcast = NewBroadcast()
	go broadcast.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}
		serverHandler(conn)
	})

	fmt.Printf("WebSocket server listening on %s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
