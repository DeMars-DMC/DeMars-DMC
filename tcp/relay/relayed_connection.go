package relay

import (
	"encoding/json"
	"net"
	"strings"
	"sync"
	"time"
)


type RelayConn struct{
	MyID string
	PeerID string //pearId
	Relay net.Conn
	Data chan *[]byte
}



func (this RelayConn ) Read(b []byte) (n int, err error) {
	data := <- this.Data
	copy(b,*data)
	return len(*data),nil
}

func (this RelayConn) Write(b []byte) (n int, err error) {
	var peerMes= RelayMessage{"Mes", this.MyID, this.PeerID, b}
	encoder := json.NewEncoder(this.Relay)
	error := encoder.Encode(peerMes)
	return len(b),	error
}

func (this RelayConn) Close() error {
	rm := getRelayManager(this.MyID)
	delete(rm.ClientConnections, this.PeerID)

	//return this.relay.Close() we need to check if it is still used before we can close it.
	return nil
}

func (this RelayConn) LocalAddr() net.Addr {
	return this.Relay.LocalAddr()
}

func (this RelayConn) RemoteAddr() net.Addr {
	return this.Relay.RemoteAddr()
}

func (this RelayConn) SetDeadline(t time.Time) error {
	return this.Relay.SetDeadline(t)
}

func (this RelayConn) SetReadDeadline(t time.Time) error {
	return this.Relay.SetDeadline(t)
}

func (this RelayConn) SetWriteDeadline(t time.Time) error {
	return this.Relay.SetWriteDeadline(t)
}

func splitAddress(address string)(nodeId, ipAddress string) {
	slist := strings.Split(address,"||")
	if len(slist) == 2{
		return slist[0],slist[1]
	}else{
		return "",""
	}
}

func DialRelayed(network,myID, peerId, relayAddress string) (net.Conn, error) {

	rm := getRelayManager(myID)
	//fmt.Println("Key ..." , relayAddress)
	if conn, ok := rm.RelayConnections[relayAddress]; ok {
		//fmt.Println("connection found")
		relayConn := RelayConn{myID,peerId, *conn, make(chan *[]byte)}
		rm.ClientConnections[peerId] = &relayConn
		return relayConn, nil
	}else {
		//else connct
		//fmt.Println("No connection found. Connecting again.", rm.RelayConnections, conn)
		connection, error := net.Dial("tcp", relayAddress)
		//register connectio to map
		if error != nil {
			return nil, error
		}else{
			relayConn := RelayConn{myID, peerId, connection, make(chan *[]byte)}
			registerWithRelay(myID, connection)
			rm.RelayConnections[relayAddress] = &connection
			rm.ClientConnections[peerId] = &relayConn
			go rm.listern(&connection)
			return relayConn, error
		}

	}


}



func  registerWithRelay(myId string, conn net.Conn){
	var regMessage = RelayMessage{"Reg",myId, myId,nil}
	encoder := json.NewEncoder(conn)
	encoder.Encode(regMessage)
	//fmt.Println("Reg message sent")
}
// Singleton object to keep a list of all connected relays. each relay can have a connection to more than one peer so the tcp connection will be reused.
type RelayManager struct{
	MyId string
	RelayConnections map[string]*net.Conn //the key can be the address or maybe we should get an Id from the relay.
	ClientConnections map[string]*RelayConn
}

// Listerns for new messages from each relay server and passes it to the coresponding RelayCon
func (rm RelayManager) listern(connection *net.Conn){

	decoder := json.NewDecoder(*connection)
	for{

		var message RelayMessage
		error := decoder.Decode(&message)
		if error != nil {

		} else{

			clientCon := rm.ClientConnections[message.FromKey]
			if clientCon == nil{
				clientCon = rm.ClientConnections[message.ToKey]
			}
			//fmt.Println("message from ", message.MyKey,clientCon )
			clientCon.Data <- &message.Message

		}
	}
}



var instance *RelayManager
var once sync.Once

func getRelayManager(myID string) *RelayManager {
	once.Do(func() {
		instance = &RelayManager{myID,make(map[string]*net.Conn),make(map[string]*RelayConn)}
	})
	return instance
}


