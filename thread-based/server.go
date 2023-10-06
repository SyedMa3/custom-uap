package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"strconv"
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
	timer := time.NewTimer(60 * time.Second)
	state := 0
	var sessionID uint32
	var addr *net.UDPAddr
	seqNum := uint32(0)

	for {
		select {
		case <-timer.C:
			fmt.Println("Session timeout")
			newM := NewUAPMessage(0x03, 0, sessionID, "")
			sendMessageChan <- &SendingMessage{*newM, addr}
			newM2 := NewUAPMessage(0x00, 0, sessionID, "")
			c <- &SendingMessage{*newM2, nil}
			fmt.Printf("%v [%v] GOODBYE from client\n", sessionID, seqNum)
			return
		case rm := <-c:
			m := rm.m
			sessionID = m.sessionID
			addr = rm.addr
			magic := m.magic
			if magic != 0xC461 {
				fmt.Println("Invalid magic number")
				newM := NewUAPMessage(0x00, 0, sessionID, "")
				c <- &SendingMessage{*newM, nil}
				return
			}
			if seqNum-1 == m.sequenceNumber {
				fmt.Println("duplicate packet")
				continue
			} else if m.sequenceNumber > seqNum {
				for i := seqNum + 1; i < m.sequenceNumber; i++ {
					fmt.Println("lost packet")
				}
			}
			seqNum = m.sequenceNumber

			command := m.command

			if state == 0 {
				if command != 0x00 {
					fmt.Println("Invalid command")
					newM := NewUAPMessage(0x00, 0, sessionID, "")
					c <- &SendingMessage{*newM, nil}
					return
				}
				fmt.Printf("%v [%v] Session Created\n", sessionID, seqNum)
				state = 1
				newM := NewUAPMessage(0x00, 0, sessionID, "")

				sendMessageChan <- &SendingMessage{*newM, rm.addr}

			} else if state == 1 {
				if command == 0x02 {
					state = 1
					continue
				} else if command == 0x01 {
					fmt.Printf("%v [%v] %v\n", sessionID, seqNum, m.data)
					newM := NewUAPMessage(0x02, 0, sessionID, "")
					sendMessageChan <- &SendingMessage{*newM, rm.addr}
				} else if command == 0x03 {
					state = 2
					newM2 := NewUAPMessage(0x00, 0, sessionID, "")
					c <- &SendingMessage{*newM2, nil}
					fmt.Printf("%v [%v] GOODBYE from client\n", sessionID, seqNum)
					return
				} else {
					fmt.Println("Invalid command")
					c <- nil
					return
				}
			}
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

		ch <- &SendingMessage{*receivedMessage, addr}
	}
}

func sendMessages(udpServer *net.UDPConn, ch chan *SendingMessage) {
	seqNum := 0
	for x := range ch {
		message := x.m
		message.sequenceNumber = uint32(seqNum)
		addr := x.addr
		seqNum++

		_, err := udpServer.WriteToUDP(message.Encode(), addr)
		if err != nil {
			continue
		}
	}
}

func main() {

	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	messageChan := make(chan *SendingMessage)
	sendMessageChan := make(chan *SendingMessage)

	cases := []reflect.SelectCase{}
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(messageChan)})
	udpAddr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	fmt.Println("Waiting on Port", port)
	if err != nil {
		log.Fatal(err)
	}
	defer udpServer.Close()
	go listenMessages(udpServer, messageChan)
	go sendMessages(udpServer, sendMessageChan)
	// defer close(sendMessageChan)

	m := make(map[uint32]chan *SendingMessage)

	for {
		index, value, recvOK := reflect.Select(cases)

		if !recvOK {
			continue
		}

		if (reflect.TypeOf(value.Interface()) == reflect.TypeOf(&SendingMessage{})) {
			receievedRM := value.Interface().(*SendingMessage)
			if receievedRM.addr == nil {
				delete(m, receievedRM.m.sessionID)
				removeElementFromSlice(cases, index) //TODO: Improve performance
				continue
			}
			receivedMessage := receievedRM.m

			if _, ok := m[receivedMessage.sessionID]; !ok {
				c := make(chan *SendingMessage)
				m[receivedMessage.sessionID] = c
				cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(c)})
				go handleSession(c, sendMessageChan)
			}
			c := m[receivedMessage.sessionID]
			c <- &SendingMessage{receivedMessage, receievedRM.addr}

			continue
		}
	}

}
