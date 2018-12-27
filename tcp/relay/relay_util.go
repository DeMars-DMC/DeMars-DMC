package relay


type RelayMessage struct {
	Type      string
	FromKey     string
	ToKey   string
	Message   []byte
}


