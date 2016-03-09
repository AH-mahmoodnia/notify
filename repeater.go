package main

import "log"

type Repeater struct {
	update    <-chan Message
	admin     chan AdminEvent
	listeners map[*Listener]chan<- Message
}

func (r Repeater) Run() {
	for {
		select {
		case msg := <-r.update:
			for _, c := range r.listeners {
				c <- msg
			}
		case msg := <-r.admin:
			switch msg.Type {
			case 0:
				log.Println("Adding listener.")
				r.listeners[msg.Listener] = msg.UpdateChan
			case 1:
				log.Println("Removing listener.")
				delete(r.listeners, msg.Listener)
			default:
				panic("unsupported admin type")
			}
		}
	}
}

func (r *Repeater) Register() *Listener {
	updateChan := make(chan Message)
	listener := &Listener{Updates: updateChan}
	r.admin <- AdminEvent{Type: 0, Listener: listener, UpdateChan: updateChan}
	return listener
}

func (r *Repeater) Unregister(listener *Listener) {
	r.admin <- AdminEvent{Type: 1, Listener: listener}
}

type AdminEvent struct {
	Type       uint8
	Listener   *Listener
	UpdateChan chan<- Message
}

type Listener struct {
	Updates <-chan Message
}
