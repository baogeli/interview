package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	url := "ws://localhost:8080/ws"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	fmt.Printf("Connected to %s\n", url)

	go func() {
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("read error: %v", err)
				return
			}
			fmt.Printf("[%s] < %s\n", time.Now().Format("15:04:05"), string(msg))
			_ = msgType
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(text)); err != nil {
			log.Printf("write: %v", err)
			return
		}
	}
}
