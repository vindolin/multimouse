package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/akamensky/argparse"
	"github.com/gorilla/websocket"
)

// this struct stores the location of an IP address
type mmrecord struct {
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// handler is the main websocket handler
func handler(w http.ResponseWriter, r *http.Request, pool *wsPool) {
	conn, err := upgrader.Upgrade(w, r, nil)
	log.Println("New connection from:", conn.RemoteAddr())
	if err != nil {
		log.Println(err)
		return
	}

	pool.Add(conn)
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
		DarkMode bool
	}
	// create a new pool and start it
	pool := WsPool()
	go pool.Start()

	go func() {
	}()

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
