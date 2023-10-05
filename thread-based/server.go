package main

import (
	"encoding/binary"
	"log"
	"net"
	"reflect"
	"time"
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

type SendingMessage struct {
	m    UAPMessage
	addr *net.UDPAddr
}

func removeElementFromSlice[T any](slice []T, index int) []T {
	newSlice := append(slice[:index], slice[index+1:]...)
	return newSlice
} //TODO Make a optimized algorithm for index allocation

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

func handleSession(c chan *SendingMessage, sendMessageChan chan *SendingMessage) {
	defer close(c)
	timer := time.NewTimer(10 * time.Second)
	state := 0

	for {
		select {
		case <-timer.C:
			log.Println("Session timeout")
			c <- nil
			return
		case rm := <-c:
			m := rm.m
			magic := m.magic
			if magic != 0xC461 {
				log.Println("Invalid magic number")
				continue
			}

			command := m.command

			if state == 0 {
				if command != 0x00 {
					log.Println("Invalid command")
					c <- nil
					return
				}
				log.Println("Session started")
				state = 1
				newM := NewUAPMessage(0x00, 0, m.sessionID, "")

				sendMessageChan <- &SendingMessage{*newM, rm.addr}

			} else if state == 1 {
				if command == 0x02 {
					state = 1
					continue
				} else if command == 0x01 {
					data := m.data
					log.Println(data)
				} else if command == 0x03 {
					state = 2
					c <- nil
					return
				} else {
					log.Println("Invalid command")
					c <- nil
					return
				}
			}
			log.Println(m)
		}
	}

}

func listenMessages(udpServer *net.UDPConn, ch chan *SendingMessage) {
	defer close(ch)
	for {
		buf := make([]byte, 1024)
		n, addr, err := udpServer.ReadFromUDP(buf)
		if err != nil {
			log.Fatal(err)
		}
		receivedMessage := &UAPMessage{}
		receivedMessage.Decode(buf[:n])

		data := string(buf[12:n])

		if data == "q" {
			break
		}

		ch <- &SendingMessage{*receivedMessage, addr}
	}
}

func sendMessages(udpServer *net.UDPConn, ch chan *SendingMessage) {
	seqNum := 0
	for x := range ch {
		message := x.m
		addr := x.addr
		seqNum++

		_, err := udpServer.WriteToUDP(message.Encode(), addr)
		if err != nil {
			continue
		}
	}
}

func main() {

	messageChan := make(chan *SendingMessage)
	sendMessageChan := make(chan *SendingMessage)

	cases := []reflect.SelectCase{}
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(messageChan)})
	udpAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 8888}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer udpServer.Close()
	go listenMessages(udpServer, messageChan)
	go sendMessages(udpServer, sendMessageChan)
	defer close(sendMessageChan)

	m := make(map[uint32]chan *SendingMessage)

	for {
		index, value, recvOK := reflect.Select(cases)

		if !recvOK {
			continue
		}

		if (reflect.TypeOf(value.Interface()) == reflect.TypeOf(&SendingMessage{})) {
			receievedRM := value.Interface().(*SendingMessage)
			receivedMessage := receievedRM.m
			log.Println(receivedMessage)

			if _, err := m[receivedMessage.sessionID]; err {
				c := make(chan *SendingMessage)
				m[receivedMessage.sessionID] = c
				cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(c)})
				go handleSession(c, sendMessageChan)
			}
			c := m[receivedMessage.sessionID]
			c <- &SendingMessage{receivedMessage, receievedRM.addr}

			continue
		}

		delete(m, value.Interface().(uint32))
		removeElementFromSlice(cases, index) //TODO Improve performance

		// c := make(chan *UAPMessage)

	}

}
