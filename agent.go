package main

import (
	"log"
	"net"
	"net/rpc"
)

func handleAgent(addr string, report chan<- Message) {
	agent := Agent{update: report}
	rpc.Register(&agent)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln("Cannot open port for agent listener:", err.Error())
		panic("should not reach this")
	}
	log.Println("Listening for agents on " + addr)
	for {
		client, err := listener.Accept()
		if err != nil {
			log.Println("Failed to handle client connection:", err.Error())
			continue
		}
		go rpc.ServeConn(client)
	}
}

type Agent struct {
	update chan<- Message
}

type MessageType uint

const (
	_ MessageType = iota
	StartMessage
	DoneMessage
	UpdateMessage
)

type Message struct {
	Id      int64
	Type    MessageType
	Message string
	Notify  bool
}

func (a Agent) Start(id int64, result *bool) error {
	a.update <- Message{Id: id, Type: StartMessage, Notify: true}
	return nil
}

type RpcUpdateMessage struct {
	Id      int64
	Message string
}

func (a Agent) Update(up *RpcUpdateMessage, result *bool) error {
	a.update <- Message{Id: up.Id, Type: UpdateMessage, Message: up.Message}
	return nil
}

func (a Agent) Done(id int64, result *bool) error {
	a.update <- Message{Id: id, Type: DoneMessage, Notify: true}
	return nil
}
