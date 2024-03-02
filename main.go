package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/akamensky/argparse"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var clientId int32 = 0
var currentClientId int32 = 0

// handler is the main websocket handler
func handler(w http.ResponseWriter, r *http.Request, pool *wsPool) {
	conn, err := upgrader.Upgrade(w, r, nil)
	log.Println("New connection from:", conn.RemoteAddr())
	if err != nil {
		log.Println(err)
		return
	}

	// Assign and increment clientId for each new connection
	currentClientId = atomic.AddInt32(&clientId, 1)
	clientId++

	pool.Add(conn)

	// Listen for incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		// Parse the incoming message as a mouse event
		var mouseData struct {
			ClientId int `json:"clientId"`
			X        int `json:"x"`
			Y        int `json:"y"`
		}
		if err := json.Unmarshal(message, &mouseData); err != nil {
			log.Println("error unmarshalling message:", err)
			continue
		}

		// Handle the mouse event...
		log.Printf("%d: %d, %d\n", mouseData.ClientId, mouseData.X, mouseData.Y)
	}
}

func main() {
	// setup command line arguments
	parser := argparse.NewParser("run", "run multimouse server")
	// mandatory arguments

	// optional arguments
	port := parser.String("p", "port",
		&argparse.Options{Required: false, Help: "port to listen on", Default: "8180"})

	const PING_INTERVAL = 10

	// parse the command line arguments
	err := parser.Parse(os.Args)
	if err != nil {
		log.Print(parser.Usage(err))
		os.Exit(1)
	}

	// this struct holds the data that will be passed to the index.html template
	type IndexTemplateData struct {
		ClientId int
	}
	// create a new pool and start it
	pool := WsPool()
	go pool.Start()

	// send a ping every n seconds
	// this is used on the javascript side to keep the websocket alive
	go func() {
		for {
			time.Sleep(PING_INTERVAL * time.Second)

			var payload = fmt.Sprintf(
				"ping %d", time.Now().Unix())
			pool.broadcast <- payload
		}
	}()

	// serve the favicon.ico file
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "favicon.ico")
	})

	// serve the index.html file
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		template.Must(template.ParseFiles("index.html")).Execute(
			w, IndexTemplateData{ClientId: int(currentClientId)})
	})

	// serve the websocket
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, pool)
	})

	log.Println("Listening on :" + *port)

	// start the http server
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
