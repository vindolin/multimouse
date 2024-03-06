package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/akamensky/argparse"
	"github.com/gorilla/websocket"
)

const (
	PING_INTERVAL = 10 * time.Second
)

type IndexTemplateData struct {
	ClientId int
}

func spinner() func() {
	symbols := []rune("-\\|/")
	i := 0
	return func() {
		fmt.Print("\033[1D\033[K" + string(symbols[i]))
		i = (i + 1) % len(symbols)
	}
}

var spin = spinner()
var noSpinner = false

func handleConnection(conn *websocket.Conn, pool *wsPool) {

	// Listen for incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				// If it's a timeout error, log it and continue reading messages
				log.Println("read timeout:", err)
				continue
			} else {
				// For other errors, log them and break out of the loop
				log.Println("read:", err)
				break
			}
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
		//fmt.Print(".")

		if !noSpinner {
			spin()
		}

		// Convert the message to a string before broadcasting
		pool.broadcast <- string(message)
	}
}

// handler is the main websocket handler
func handler(w http.ResponseWriter, r *http.Request, pool *wsPool) {
	var upgrader = websocket.Upgrader{
		// ReadBufferSize:  1024,
		// WriteBufferSize: 1024,
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
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

//go:embed index.html static
var static embed.FS

func main() {
	// setup command line arguments
	parser := argparse.NewParser("run", "run multimouse server")
	// mandatory arguments

	// optional arguments
	port := parser.String("p", "port",
		&argparse.Options{Required: false, Help: "port to listen on", Default: "8180"})

	noSpinner = *parser.Flag("n", "no-spinner",
		&argparse.Options{Required: false, Help: "disable spinner"})

	// parse the command line arguments
	err := parser.Parse(os.Args)
	if err != nil {
		log.Print(parser.Usage(err))
		os.Exit(1)
	}

	// create a new pool of websockets and start it
	pool := WsPool()
	go pool.Start()

	// send a ping every n seconds
	// this is used on the javascript side to keep the websocket alive
	go func() {
		for {
			time.Sleep(PING_INTERVAL)

			var payload = fmt.Sprintf(
				"ping %d", time.Now().Unix())
			pool.broadcast <- payload
		}
	}()

	if false { // serve the static files from the normal filesystem
		fs := http.FileServer(http.Dir("./static"))
		http.Handle("/static/", http.StripPrefix("/static/", fs))

		// serve the index.html file
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			template.Must(template.ParseFiles("index.html")).Execute(
				w, IndexTemplateData{})
		})

	} else { // embed the static files into the binary
		fs := http.FS(static)
		http.Handle("/static/", http.FileServer(fs))

		// serve the index.html file
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			tmpl, err := template.ParseFS(static, "index.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tmpl.Execute(w, IndexTemplateData{})
		})
	}

	// serve the websocket
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, pool)
	})

	log.Println("Listening on :" + *port)

	// start the http server
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
