package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return ws, err
	}
	return ws, nil
}

func Reader(conn *websocket.Conn) {
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		fmt.Println(string(p))

		if err := conn.WriteMessage(messageType, p); err != nil {
			log.Println(err)
			return
		}
	}
}

func Writer(conn *websocket.Conn) {
	for {
		fmt.Println("Sending")
		messageType, r, err := conn.NextReader()
		if err != nil {
			fmt.Println(err)
			return
		}
		w, err := conn.NextWriter(messageType)
		if err != nil {
			fmt.Println(err)
			return
		}
		if _, err := io.Copy(w, r); err != nil {
			fmt.Println(err)
			return
		}
		if err := w.Close(); err != nil {
			fmt.Println(err)
			return
		}
	}
}

type Client struct {
	ID   string
	Conn *websocket.Conn
	Pool *Pool
}

type Message struct {
	Type int         `json:"type"`
	Body BaseMessage `json:"body"`
}

type BaseMessage struct {
	Action string `json:"action"`
	Body   string `json:"body"`
}

type PingReturnMessage struct {
	Action string `json:"action"`
	Body   string `json:"body"`
}

func (c *Client) Read() {
	defer func() {
		c.Pool.Unregister <- c
		c.Conn.Close()
	}()

	for {
		_, p, err := c.Conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		//var incomingMessage BaseMessage
		var result map[string]interface{}
		errJSON := json.Unmarshal(p, &result)
		if errJSON != nil {
			fmt.Printf("There was an error decoding the json. err = %s", err)
			return
		}
		//fmt.Println(result)
		if result["action"] == "ping" {
			//fmt.Println(result)
			time.Sleep(time.Millisecond * 100)

			//returnJSON, _ := json.Marshal(result)
			//fmt.Println(returnJSON)
			messageReturn := Message{
				Type: 0,
				Body: BaseMessage{Action: "ping", Body: result["body"].(string)},
			}
			c.Conn.WriteJSON(messageReturn)
		} else {
			fmt.Println(result["action"])
			message := Message{}
			c.Pool.Broadcast <- message
			fmt.Printf("Message Received: %+v\n", message)

		}

	}
}

type Pool struct {
	Register   chan *Client
	Unregister chan *Client
	Clients    map[*Client]bool
	Broadcast  chan Message
}

func NewPool() *Pool {
	return &Pool{
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan Message),
	}
}

func (pool *Pool) Start() {
	for {
		select {
		case client := <-pool.Register:
			pool.Clients[client] = true
			fmt.Println("Size of Connection Pool: ", len(pool.Clients))
			for client, _ := range pool.Clients {
				//fmt.Println(client)
				loginMessage := Message{
					Type: 0,
					Body: BaseMessage{Action: "Login of User"},
				}
				client.Conn.WriteJSON(loginMessage)
			}
			break
		case client := <-pool.Unregister:
			delete(pool.Clients, client)
			fmt.Println("Size of Connection Pool: ", len(pool.Clients))
			for client, _ := range pool.Clients {
				logoutMessage := Message{
					Type: 0,
					Body: BaseMessage{Action: "Logout of User"},
				}
				client.Conn.WriteJSON(logoutMessage)
			}
			break
		case message := <-pool.Broadcast:
			fmt.Println("Sending message to all clients in Pool")
			for client, _ := range pool.Clients {
				if err := client.Conn.WriteJSON(message); err != nil {
					fmt.Println(err)
					return
				}
			}
		}
	}
}
