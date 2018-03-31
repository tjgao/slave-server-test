package main

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WorkContext struct {
	Conn         *websocket.Conn
	RunningTasks *sync.WaitGroup
	Exiting      chan struct{}
	dataOut      chan string
}

func newContext(conn *websocket.Conn) *WorkContext {
	var wg sync.WaitGroup
	return &WorkContext{
		Conn:         conn,
		RunningTasks: &wg,
		Exiting:      make(chan struct{}),
		dataOut:      make(chan string),
	}
}

func (w *WorkContext) serve() {
	// start writer goroutine
	go func() {
	OUTSIDE_LOOP:
		for {
			select {
			case text := <-w.dataOut:
				w.Conn.WriteMessage(websocket.BinaryMessage, []byte(text))
			case <-w.Exiting:
				break OUTSIDE_LOOP
			}
		}
	}()

	w.Conn.SetReadDeadline(time.Now().Add(time.Second * 2))
OUTSIDE_LOOP2:
	for {
		message := Message{}
		err := w.Conn.ReadJSON(&message)
		if err != nil {
			log.Println("read error: ", err)
			break
		} else {
			w.onMessage(&message)
		}

		select {
		case <-w.Exiting:
			break OUTSIDE_LOOP2
		default:
		}
	}
}

func (w *WorkContext) onMessage(msg *Message) {
	switch msg.ID {
	case RegisterRespType:
		var obj RegisterResp
		err := json.Unmarshal([]byte(msg.Body), &obj)
		if err != nil {
			log.Println("invalid json data: ", msg.Body)
			break
		}
		if !w.onRegisterResp(&obj) {
			close(w.Exiting)
		}
	case TaskRequestType:
		var obj Task
		err := json.Unmarshal([]byte(msg.Body), &obj)
		if err != nil {
			log.Println("invalid json data: ", msg.Body)
			break
		}
		w.onTaskRequest(&obj)
	default:
		log.Printf("Unknown message: %d, %s", msg.ID, msg.Body)
	}
}

func (w *WorkContext) onTaskRequest(req *Task) {
	// start a goroutine to grab the data
	go func() {
		w.RunningTasks.Add(1)
		defer w.RunningTasks.Done()
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		var result TaskResult
		result.TransactionID = req.TransactionID
		client := &http.Client{Transport: tr}
		resp, err := client.Get(req.TargetURL)
		if err != nil {
			log.Println("Failed to access url: ", req.TargetURL)
			result.Code = FailedToAccessURL
			result.Description = "Failed to access url: " + req.TargetURL
		} else {
			b, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Printf("Failed to read data from http response: %v\n", err)
				result.Code = FailedToReadFromResponse
				result.Description = "Failed to read from http response"
			} else {
				result.Result = string(b)
				result.Code = RetrieveDataSuccessfully
				result.Description = "OK"
			}
		}
		bs, err := json.Marshal(result)
		if err != nil {
			log.Println("Serious problem, json marshal operation failed")
		} else {
			w.dataOut <- string(bs)
		}
	}()
}

func (w *WorkContext) onRegisterResp(resp *RegisterResp) bool {
	if resp.Code != 0 {
		log.Println("registration rejected: ", resp.Description)
		return false
	}
	return true
}

func (w *WorkContext) waitTasksDone(t time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		w.RunningTasks.Wait()
	}()

	select {
	case <-c:
		return true
	case <-time.After(t):
		return false
	}
}

func (w *WorkContext) close() {
	w.Conn.Close()
}
