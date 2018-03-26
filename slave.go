package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	u := flag.String("u", "", "The url for registration")

	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	log.Printf("connecting to url(%s)", *u)

	tm := time.NewTicker(time.Second * 5)
	defer tm.Stop()
	// In case sometimes server may be down, client should
	// keep trying at a reasonable rate
	var conn *websocket.Conn
	for {
		select {
		case <-tm.C:
			c, _, err := websocket.DefaultDialer.Dial(*u, nil)
			if err == nil {
				conn = c
				tm.Stop()
				break
			} else {
				log.Println("Failed to connect, wait 5 seconds and try again : ", err)
			}
		case <-interrupt:
			log.Println("Interrupted by user, exit.")
			return
		}
	}

	done := make(chan struct{})

	go func() {
		defer conn.Close()
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	// for {
	// 	select {
	// 	case t := <-ticker.C:
	// 		err := conn.WriteMessage(websocket.TextMessage, []byte(t.String()))
	// 		if err != nil {
	// 			log.Println("write:", err)
	// 			return
	// 		}
	// 	case <-interrupt:
	// 		log.Println("interrupt")
	// 		err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	// 		if err != nil {
	// 			log.Println("write close:", err)
	// 			return
	// 		}
	// 		select {
	// 		case <-done:
	// 		case <-time.After(time.Second):
	// 		}
	// 		conn.Close()
	// 		return
	// 	}
	// }
}
