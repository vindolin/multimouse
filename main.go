package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/akamensky/argparse"
	"github.com/gorilla/websocket"
)

const (
	PING_INTERVAL = 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type IndexTemplateData struct {
	ClientId int
}

func handleConnection(conn *websocket.Conn, pool *wsPool) {
	// Listen for incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			pool.Remove(conn)
			break
		}

		// Parse the incoming message as a mouse event
		var mouseData struct {
			ClientId int     `json:"clientId"`
			X        float64 `json:"x"`
			Y        float64 `json:"y"`
		}
		if err := json.Unmarshal(message, &mouseData); err != nil {
			log.Println("error unmarshalling message:", err)
			pool.Remove(conn)
			break
		}

		// Convert the message to a string before broadcasting
		pool.broadcast <- string(message)
	}
}

func main() {
	// setup command line arguments
	parser := argparse.NewParser("run", "run multimouse server")

	// optional arguments
	port := parser.String("p", "port",
		&argparse.Options{Required: false, Help: "port to listen on", Default: "8180"})

	// parse the command line arguments
	err := parser.Parse(os.Args)
	if err != nil {
		log.Print(parser.Usage(err))
		os.Exit(1)
	}

	// create a new pool and start it
	pool := WsPool()
	go pool.Start()

	// send a ping every n seconds
	go func() {
		for {
			time.Sleep(PING_INTERVAL * time.Second)

			var payload = fmt.Sprintf(
				"ping %d", time.Now().Unix())
			pool.broadcast <- payload
		}
	}()

	// create an upgrader
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// handler is the main websocket handler
	handler := func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		log.Println("New connection from:", conn.RemoteAddr())
		if err != nil {
			log.Println(err)
			return
		}

		pool.Add(conn)

		handleConnection(conn, pool)
	}

	// serve the websocket
	http.HandleFunc("/ws", handler)

	log.Println("Listening on :" + *port)

	// start the http server
	err = http.ListenAndServe(":"+*port, nil)
	if err != nil {
		log.Println(err)
		return
	}
}
