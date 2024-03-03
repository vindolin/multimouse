package main

import (
	"encoding/json"
	"fmt"
	"html/template"
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

type IndexTemplateData struct {
	ClientId int
}

func handleConnection(conn *websocket.Conn, pool *wsPool) {
	// Listen for incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
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
			continue
		}

		// Handle the mouse event...
		// log.Printf("%d: %f, %f\n", mouseData.ClientId, mouseData.X, mouseData.Y)

		// Convert the message to a string before broadcasting
		pool.broadcast <- string(message)
	}
}

// handler is the main websocket handler
func handler(w http.ResponseWriter, r *http.Request, pool *wsPool) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	log.Println("New connection from:", conn.RemoteAddr())
	if err != nil {
		log.Println(err)
		return
	}

	pool.Add(conn)

	handleConnection(conn, pool)
}

func main() {
	// setup command line arguments
	parser := argparse.NewParser("run", "run multimouse server")
	// mandatory arguments

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

	// serve the cursor.png file
	http.HandleFunc("/cursor.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "cursor.png")
	})

	// serve the index.html file
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		template.Must(template.ParseFiles("index.html")).Execute(
			w, IndexTemplateData{})
	})

	// serve the websocket
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, pool)
	})

	log.Println("Listening on :" + *port)

	// start the http server
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
