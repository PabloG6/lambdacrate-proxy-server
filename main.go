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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			fmt.Println("error occurred when attempting to close socket. ")
		}
	}()
}
