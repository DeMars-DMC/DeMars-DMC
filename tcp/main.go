package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"tcp/relay"
)

type ClientOne struct {
	key    string
	socket net.Conn
	data   chan []byte
}



//type NATAddr struct {
//	PublicAddr string
//	LocalAddr  string
//}
//
//type table struct {
//	m map[string]NATAddr
//}
//
//type Message struct {
//	Type      string
//	Key       int
//	ToKey     int
//	Addresses *NATAddr
//}
//
//
//
func (client *ClientOne) receive() {
	//decoder := json.NewDecoder(relayReader)
	for {
		buf := make([]byte, 2048)
		_, erro := client.socket.Read(buf)
		//err := decoder.Decode(&message)

		if erro != nil {

			break
		}
		//switch message.Type {
		//case "peer":
			fmt.Println("Message form ",client.key, " : ", string(buf))
		//
		//}
	}
}
//
//func (client *Client) register(){
//	var regMessage = Message{"Reg",client.key, 999,nil}
//	encoder := json.NewEncoder(client.socket)
//	encoder.Encode(regMessage)
//	fmt.Println("Reg message sent")
//}
//
//func (client * Client) connectToPeer(id int){
//	var connectMessage = Message{"Con",client.key, id,nil}
//	encoder := json.NewEncoder(client.socket)
//	encoder.Encode(connectMessage)
//}
//
//
//func startClientListener(){
//
//}
//
func startClientMode(key *string,bootstrap *string) {
	fmt.Println("Starting client...")

		connection, error := relay.DialRelayed("tcp", *key, *key, *bootstrap)
		client := &ClientOne{key: *key, socket: connection}
	if error != nil {
				fmt.Println(error)
			}
	go client.receive()
	for {
				reader := bufio.NewReader(os.Stdin)
				message, _ := reader.ReadString('\n')

		slist := strings.Split(message,"||")
		if len(slist) >= 3 {
			connection2, e := relay.DialRelayed("tcp", *key, slist[0], slist[1])
			if e != nil {
				fmt.Println(e)
			}

			client2 := &ClientOne{key: *key, socket: connection2}
			go client2.receive()
			b := []byte(slist[2])
			_, ee := client2.socket.Write(b)
			fmt.Println(ee)
		}
				//connection.Write([]byte(strings.TrimRight(message, "\n")))
			}
}
//	fmt.Println("Starting client...")
//
//	connection, error := net.Dial("tcp", *bootstrap)
//
//	if error != nil {
//		fmt.Println(error)
//	}
//	fmt.Println("connected")
//	client := &Client{key: *id, socket: connection}
//	client.register()
//	go client.receive()
//	for {
//		reader := bufio.NewReader(os.Stdin)
//		message, _ := reader.ReadString('\n')
//
//		var i int
//		if _, err := fmt.Sscan(message, &i); err == nil {
//			client.connectToPeer(i)
//		}else {
//			fmt.Println("nto a int ", message)
//		}
//		//connection.Write([]byte(strings.TrimRight(message, "\n")))
//	}
//}

func main() {
	flagMode := flag.String("mode", "server", "start in client or server mode")
	flagKey := flag.String("key", "999","key for client or server")
	port := flag.Int("port", 9999, "server port")
	//flagId := flag.Int("id", 9999, "start in client or server mode")
	bootstrap := flag.String("bootstrap", "localhost:9999", "servername/IP:port")
	flag.Parse()
	if strings.ToLower(*flagMode) == "server" {
		relay.StartServerMode(*flagKey, *port)
	}else {
		startClientMode(flagKey, bootstrap)
	}
}
