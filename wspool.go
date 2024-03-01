package main

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// wsPool is a collection of websocket connections
type wsPool struct {
	clients   sync.Map
	broadcast chan string
}

func WsPool() *wsPool {
	return &wsPool{
		broadcast: make(chan string),
	}
}

func (p *wsPool) Add(conn *websocket.Conn) {
	p.clients.Store(conn, true)
}

func (p *wsPool) Remove(conn *websocket.Conn) {
	p.clients.Delete(conn)
}

func (p *wsPool) Broadcast(ip string) {
	p.broadcast <- ip
}

func (p *wsPool) Start() {
	for {
		ip := <-p.broadcast
		p.clients.Range(func(client, _ interface{}) bool {
			err := client.(*websocket.Conn).WriteMessage(
				websocket.TextMessage, []byte(ip))
			if err != nil {
				log.Println(err)
				p.Remove(client.(*websocket.Conn))
			}
			return true
		})
	}
}
