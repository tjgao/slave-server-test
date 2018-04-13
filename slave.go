package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	logLevelTable := map[string]log.Level{
		"panic": log.PanicLevel,
		"error": log.ErrorLevel,
		"warn":  log.WarnLevel,
		"info":  log.InfoLevel,
		"debug": log.DebugLevel,
	}

	u := flag.String("u", "", "The url for registration")
	logLevel := flag.String("l", "info", "specify log level, available levels are: panic, error, warn, info and debug")
	flag.Parse()

	if *u == "" {
		log.Fatal("A valid url is required!")
	}

	_, err := url.ParseRequestURI(*u)
	if err != nil {
		log.Fatal("Invalid url")
	}

	if level, ok := logLevelTable[*logLevel]; ok {
		log.SetLevel(level)
	} else {
		log.Warn("unrecognized log level specified, use warn level instead")
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	tm := time.NewTicker(time.Second * 5)
	defer tm.Stop()

	// In case sometimes server may be down, client should
	// keep trying at a reasonable rate
	var conn *websocket.Conn
	log.Info("connecting to url: ", *u)
WAITLOOP:
	for {
		select {
		case <-tm.C:
			c, _, err := websocket.DefaultDialer.Dial(*u, nil)
			if err == nil {
				conn = c
				log.Info("Connected!")
				break WAITLOOP
			} else {
				log.Error("Failed to connect, wait 5 seconds and try again. ", err)
			}
		case <-interrupt:
			log.Info("Interrupted by user, exit.")
			return
		}
	}

	ctx := newContext(conn)
	defer ctx.close()

	go ctx.serve()

	// main goroutine is waiting here until the user chooses to exit
	select {
	case <-interrupt:
		log.Info("Interrupted!")
		ctx.disableRead()
		ctx.close()
		close(ctx.Exiting)
		break
	}

	// wait for all tasks being done
	log.Info("wait a few seconds to clean up ...")
	ctx.waitTasksDone(time.Second * 5)
}
