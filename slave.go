package main

import (
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	//	u := flag.String("u", "", "The url for registration")
	//	flag.Parse()
	log.SetFlags(0)

	if len(os.Args) < 2 {
		log.Fatal("A valid url is required!")
	}
	u := os.Args[1]

	_, err := url.ParseRequestURI(u)
	if err != nil {
		log.Fatal("Invalid url")
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	tm := time.NewTicker(time.Second * 5)
	defer tm.Stop()

	// In case sometimes server may be down, client should
	// keep trying at a reasonable rate
	var conn *websocket.Conn
	log.Printf("connecting to url(%s)", u)
WAITLOOP:
	for {
		select {
		case <-tm.C:
			c, _, err := websocket.DefaultDialer.Dial(u, nil)
			if err == nil {
				conn = c
				log.Println("Connected!")
				break WAITLOOP
			} else {
				log.Println("Failed to connect, wait 5 seconds and try again : ", err)
			}
		case <-interrupt:
			log.Println("Interrupted by user, exit.")
			return
		}
	}

	ctx := newContext(conn)
	defer ctx.close()

	go ctx.serve()

	// main goroutine is waiting here until the user chooses to exit
	select {
	case <-interrupt:
		log.Println("Interrupted by user!")
		ctx.disableRead()
		ctx.close()
		close(ctx.Exiting)
		break
	}

	// wait for all tasks being done
	log.Println("wait a few seconds to clean up ...")
	ctx.waitTasksDone(time.Second * 5)
}
