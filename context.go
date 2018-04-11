package main

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type WorkContext struct {
	conn         *websocket.Conn
	runningTasks *sync.WaitGroup
	Exiting      chan struct{}
	dataOut      chan []byte
	disable      int32 // It is always accessed atomically
}

func newContext(conn *websocket.Conn) *WorkContext {
	var wg sync.WaitGroup
	return &WorkContext{
		conn:         conn,
		runningTasks: &wg,
		Exiting:      make(chan struct{}),
		dataOut:      make(chan []byte),
	}
}

func (w *WorkContext) disableRead() {
	atomic.StoreInt32(&w.disable, 1)
}

func (w *WorkContext) serve() {
	// start writer goroutine
	go func() {
	OUTSIDE_LOOP:
		for {
			select {
			case text := <-w.dataOut:
				if text != nil {
					w.conn.WriteMessage(websocket.BinaryMessage, text)
				} else {
					w.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					select {
					case <-time.After(time.Second):
					}

					w.conn.Close()
				}
			case <-w.Exiting:
				break OUTSIDE_LOOP
			}
		}
	}()

	for {
		t, buf, err := w.conn.ReadMessage()
		if atomic.LoadInt32(&(w.disable)) != 0 {
			break
		}

		if err != nil {
			log.Println("read error: ", err)
			break
		} else if t != websocket.BinaryMessage {
			log.Println("read error: recv text message while binaries are expected")
			continue
		}

		message := Message{}
		if err := Decode(buf, &message); err != nil {
			log.Println("failed to decode message: ", err)
			continue
		} else {
			w.onMessage(&message)
		}
	}
}

func (w *WorkContext) onMessage(msg *Message) {
	switch msg.ID {
	case RegisterRespType:
		var obj RegisterResp
		err := DecodeRegisterResp(msg.Body, &obj)
		if err != nil {
			log.Println("invalid RegisterResp data: ", msg.Body)
			break
		}
		if !w.onRegisterResp(&obj) {
			close(w.Exiting)
		}
	case TaskRequestType:
		var obj Task
		err := DecodeTask(msg.Body, &obj)
		if err != nil {
			log.Println("invalid TaskReq data: ", msg.Body)
			break
		}
		w.onTaskRequest(&obj, msg.TransID)
	default:
		log.Printf("Unknown message: %d, %s", msg.ID, msg.Body)
	}
}

func (w *WorkContext) close() {
	go func() {
		w.dataOut <- nil
	}()
}

func (w *WorkContext) onTaskRequest(req *Task, transID int64) {
	// start a goroutine to grab the data
	go func() {
		w.runningTasks.Add(1)
		defer w.runningTasks.Done()
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		var result TaskResult
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
				result.Result = b
				result.Code = RetrieveDataSuccessfully
				result.Description = "OK"
			}
		}
		bs, err := EncodeTaskResult(&result)
		if err != nil {
			panic("Serious problem, json marshal operation failed")
		} else {
			msg := Message{
				ID:      TaskResultType,
				TransID: transID,
				Body:    bs,
			}

			bytes, err := Encode(&msg)
			if err != nil {
				panic("Serious problem, json marshal operation failed")
			} else {
				w.dataOut <- bytes
			}
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
		w.runningTasks.Wait()
	}()

	select {
	case <-c:
		return true
	case <-time.After(t):
		return false
	}
}
