package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	http.HandleFunc("/ws", handleConnection)
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal("Failed ot start server. ")
	}
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	fmt.Println("hello this is a connection")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		defer func() {
			err := conn.Close()
			log.Println("closing connection")
			if err != nil {
				fmt.Println("error occurred when attempting to close socket. ")
			}
		}()

		count := 0
		for {
			count++
		}
	}()
}
