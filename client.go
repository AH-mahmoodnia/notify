package main

import (
	"bufio"
	"log"
	"math/rand"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func startClient() {
	var err error
	rand.Seed(time.Now().UnixNano())
	id := rand.Int63()
	// establish RPC connection with server
	client, err := rpc.Dial("tcp", "127.0.0.1:31416")
	if err != nil {
		log.Fatalln("Failed to connect to server:", err.Error())
		panic("should not reach this")
	}
	// start interacting with server
	var result bool
	client.Call("Agent.Start", id, &result)
	defer client.Call("Agent.Done", id, &result)
	// establish signal trap to communicate interrupts to server before exiting
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT)
	// start Stdin line reader
	updateChan := make(chan string)
	go lineReader(updateChan)
	// start event processing loop
	for {
		select {
		case <-signalChan:
			client.Call("Agent.Done", "", &result)
			return
		case line, ok := <-updateChan:
			if !ok {
				// channel closed
				return
			}
			if err = client.Call("Agent.Update", &RpcUpdateMessage{Id: id, Message: line}, &result); err != nil {
				log.Println("Error connecting to server:", err.Error())
				return
			}
		}
	}
	panic("should not reach this")
}

func lineReader(updateChan chan<- string) {
	defer close(updateChan)
	rdr := bufio.NewReader(os.Stdin)
	var err error
	var line string
	for err == nil {
		line, err = rdr.ReadString('\n')
		updateChan <- line
		if config.verbose {
			os.Stdout.WriteString(line)
		}
	}
}
