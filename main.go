package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	uuid "github.com/nu7hatch/gouuid"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Proxy struct {
	responseChan chan string
	wg           *sync.WaitGroup
	requests     map[string]chan IncomingMessage
	requestChan  chan OutgoingMessage
	conn         *websocket.Conn
	readWait     time.Duration
}

type IncomingMessage struct {
	StatusCode     int         `json:"status_code"`
	Headers        http.Header `json:"headers"`
	Body           []byte      `json:"body"`
	ContentLength  int64       `json:"content_length"`
	ConversationID string      `json:"conversation_id"`
	Error          error       `json:"error"`
}
type OutgoingMessage struct {
	Proto          string      `json:"protocol"`
	Host           string      `json:"host"`
	ConversationID string      `json:"conversation_id"`
	Headers        http.Header `json:"headers"`
	Body           []byte      `json:"body"`
	Path           string      `json:"path"`
	ContentLength  int64       `json:"content_length"`
	Method         string      `json:"method"`
}

func main() {
	proxy := &Proxy{
		requests: make(map[string]chan IncomingMessage),
		readWait: time.Minute * 2,
		wg:       &sync.WaitGroup{},
	}

	http.HandleFunc("/ws", proxy.handleConnection)
	http.HandleFunc("/", proxy.handleRequests)

	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal("Failed ot start server. ")
	}
}

func (proxy *Proxy) handleRequests(w http.ResponseWriter, r *http.Request) {

	requestId, err := uuid.NewV4()
	if err != nil {
		log.Println("failed to generate uuid for request")
	}

	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("failed to read body for request. ")
	}

	forwardRequest := OutgoingMessage{
		ConversationID: requestId.String(),
		Headers:        r.Header,
		Path:           r.URL.Path,
		Body:           bytes,
		Method:         r.Method,
		ContentLength:  r.ContentLength,
	}

	_, cancel := context.WithTimeout(r.Context(), time.Minute*10)
	defer cancel()
	channel, err := proxy.SendMessage(forwardRequest)
	if err != nil {
		log.Println("Failed to send message: aborting request")
		return
	}
	select {
	case msg := <-channel:
		{
			if errors.Is(msg.Error, syscall.ECONNREFUSED) {
				cancel()
			}
			w.WriteHeader(msg.StatusCode)
			for key, values := range msg.Headers {
				for _, v := range values {
					w.Header().Add(key, v)

				}
			}
			_, err = w.Write(msg.Body)
			if err != nil {

				log.Println()
			}

		}

	}

}
func (proxy *Proxy) handleConnection(w http.ResponseWriter, r *http.Request) {
	fmt.Println("creating new connection with client")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	conn.SetPongHandler(func(appData string) error {
		log.Println("being ponged", appData)
		return nil
	})

	conn.SetPingHandler(func(appData string) error {
		log.Println("being pinged", appData)
		readDeadline := time.Now().Add(proxy.readWait)
		err := conn.SetReadDeadline(time.Now().Add(proxy.readWait))
		log.Println("read deadline updated: ", readDeadline)

		if err != nil {
			return err
		}
		err = conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(proxy.readWait))
		if errors.Is(err, websocket.ErrCloseSent) {
			return nil
		} else if _, ok := err.(net.Error); ok {
			return nil
		}
		return err

	})

	closeChannel := make(chan bool)
	err = conn.SetReadDeadline(time.Now().Add(proxy.readWait))
	if err != nil {
		log.Println("failed to set read deadline: ", err)
	}
	if err != nil {
		log.Println("unable to set read deadline, terminating connection")
		return
	}

	proxy.conn = conn

	go proxy.readMessages()
	go func() {
		defer func() {
			err := conn.Close()
			log.Println("closing connection")
			if err != nil {
				fmt.Println("error occurred when attempting to close socket. ")
			}
		}()

		for {
			select {
			case <-closeChannel:
				{
					err = proxy.conn.Close()
				}

			default:

			}

		}
	}()
	proxy.wg.Add(1)
	proxy.wg.Wait()
	log.Println("closing ws connection")

}

func (proxy *Proxy) SendMessage(message OutgoingMessage) (response chan IncomingMessage, err error) {

	response = make(chan IncomingMessage)

	proxy.requests[message.ConversationID] = response
	err = proxy.conn.WriteJSON(message)
	return response, err
}

func (proxy *Proxy) readMessages() {
	defer proxy.wg.Done()
	for {
		readDeadline := time.Now().Add(proxy.readWait)
		log.Println(readDeadline)
		err := proxy.conn.SetReadDeadline(readDeadline)
		log.Println("read deadline updated: ", readDeadline)
		if err != nil {
			log.Println("failed to set read deadline for proxy: ", err)
		}
		messageType, message, err := proxy.conn.ReadMessage()
		if err != nil {
			log.Println("failed to read message:", err)
			break
		}

		switch messageType {
		case websocket.TextMessage:
			{

				response := &IncomingMessage{}
				err := json.Unmarshal(message, response)
				if err != nil {
					log.Println("unable to unmarshall json response")

				}
				channel := proxy.requests[response.ConversationID]
				//clear the request map to prevent memory overload

				delete(proxy.requests, response.ConversationID)
				channel <- *response
			}

		case websocket.PingMessage:
			{
				log.Println("successfully retrieved a message from client")
			}

		case websocket.CloseMessage:
			{
				log.Println("Attempting to close with message")
				err := proxy.conn.Close()
				if err != nil {
					log.Println("failed to close connection with proxy")
				}
			}

		default:
			log.Println("received some kind of message: ", messageType)
		}
	}

	log.Println("done reading messages")
}
