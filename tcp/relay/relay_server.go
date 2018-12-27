package relay

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

type ClientManager struct {
	key string
	clients    map[*Client]bool
	register   chan *Client
	setKey     chan *Client
	unregister chan *Client
	connect    chan string
	nodes      map[string]*Client
}

func StartServerMode(myKey string, port int) {
	fmt.Println("Starting server...", port)
	service := ":" + strconv.Itoa(port)
	listener, error := net.Listen("tcp", service)
	if error != nil {
		fmt.Println(error)
	}
	manager := ClientManager{
		key: 		myKey,
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		setKey:     make(chan *Client),
		unregister: make(chan *Client),
		connect:    make(chan string),
		nodes:      make(map[string]*Client),
	}
	go manager.start()
	for {
		connection, _ := listener.Accept()
		if error != nil {
			fmt.Println(error)
		}
		client := &Client{key: "999", socket: connection, data: make(chan []byte)}
		manager.register <- client
		go manager.receive(client)
		go manager.send(client)
	}
}

func (manager *ClientManager) send(client *Client) {
	defer client.socket.Close()
	for {
		select {
		case message, ok := <-client.data:
			if !ok {
				return
			}
			client.socket.Write(message)
		}
	}
}

func (manager *ClientManager) receive(client *Client) {
	for {

		decoder := json.NewDecoder(client.socket)
		var message RelayMessage
		err := decoder.Decode(&message)
		fmt.Println(message)
		if err != nil {
			manager.unregister <- client
			client.socket.Close()
			break
		}
		switch message.Type {
		case "Reg":
			client.key = message.FromKey
			manager.setKey <- client

		case "Mes":
			fmt.Println( message.ToKey, " : ", message.Message)

				if peer,ok := manager.nodes[message.ToKey];ok {
					var peerMes= RelayMessage{"Mes", message.FromKey, message.ToKey, message.Message}
					fmt.Println(peerMes)
					encoder := json.NewEncoder(peer.socket)
					encoder.Encode(peerMes)
				}

		}

	}
}



type Client struct {
	key    string
	socket net.Conn
	data   chan []byte
}

func (manager *ClientManager) start() {
	for {
		select {
		case connection := <-manager.register:
			manager.clients[connection] = true
			fmt.Println("Added new connection!")
		case connection := <-manager.unregister:
			if _, ok := manager.clients[connection]; ok {
				close(connection.data)
				delete(manager.clients, connection)
				fmt.Println("A connection has terminated!")
			}
		case connection := <-manager.setKey:
			if _, ok := manager.clients[connection]; ok {
				manager.nodes[connection.key] = connection
				fmt.Println("Client Registered with key: ", connection.key)
				peer := manager.nodes[connection.key]
				if peer != nil {
					fmt.Println("localAddr:" + peer.socket.LocalAddr().String() + "remoteAddr:" + peer.socket.RemoteAddr().String())

				} else {
					fmt.Println("Key [", connection.key, "] not found!!")
				}


			}

		}
	}
}