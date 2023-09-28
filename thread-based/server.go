package main

import (
	"encoding/binary"
	"log"
	"net"
)

type Server struct {
	listenAddr string
}

type UAPMessage struct {
	magic          uint16
	version        uint8
	command        uint8
	sequenceNumber uint32
	sessionID      uint32
	data           string
}

func NewUAPMessage(command uint8, seqNum, sessionID uint32, data string) *UAPMessage {
	return &UAPMessage{
		magic:          0xC461,
		version:        1,
		command:        command,
		sequenceNumber: seqNum,
		sessionID:      sessionID,
		data:           data,
	}
}

func (m *UAPMessage) Encode() []byte {
	dataBytes := []byte(m.data)
	message := make([]byte, 12+len(dataBytes))

	binary.BigEndian.PutUint16(message[0:2], m.magic)
	message[2] = m.version
	message[3] = m.command
	binary.BigEndian.PutUint32(message[4:8], m.sequenceNumber)
	binary.BigEndian.PutUint32(message[8:12], m.sessionID)
	copy(message[12:], dataBytes)

	return message
}

func (m *UAPMessage) Decode(message []byte) {
	m.magic = binary.BigEndian.Uint16(message[0:2])
	m.version = message[2]
	m.command = message[3]
	m.sequenceNumber = binary.BigEndian.Uint32(message[4:8])
	m.sessionID = binary.BigEndian.Uint32(message[8:12])
	m.data = string(message[12:])
}

func NewServer(listenAddr string) *Server {
	return &Server{listenAddr: listenAddr}
}

func (s *Server) Start() error {
	udpServer, err := net.ListenPacket("udp", s.listenAddr)
	if err != nil {
		return err
	}
	defer udpServer.Close()

	for {
		buf := make([]byte, 1024)
		n, addr, err := udpServer.ReadFrom(buf)
		if err != nil {
			return err
		}
		go func() {
			_, err = udpServer.WriteTo(buf[:n], addr)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}
}

func handleSession(c chan *UAPMessage) {
	// timer := time.NewTimer(10 * time.Second)
	// state := 0

}

func main() {
	udpAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 8888}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer udpServer.Close()

	m := make(map[uint32]chan *UAPMessage)

	for {
		buf := make([]byte, 1024)
		n, _, err := udpServer.ReadFrom(buf)
		if err != nil {
			log.Fatal(err)
		}

		receivedMessage := &UAPMessage{}
		receivedMessage.Decode(buf[:n])

		c := make(chan *UAPMessage)

		if _, err := m[receivedMessage.sessionID]; err {
			c := make(chan *UAPMessage)
			go handleSession(c)
		}

		c <- receivedMessage
	}

}
