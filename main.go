package main

import (
	"encoding/json"
	"flag"
	"html"
	"io"
	"log"
	"net/http"

	"golang.org/x/net/websocket"
)

var config struct {
	serve   bool
	verbose bool
}

var updateChan = make(chan Message, 1000)
var repeater = Repeater{update: updateChan, admin: make(chan AdminEvent), listeners: make(map[*Listener]chan<- Message)}

func main() {
	flag.BoolVar(&config.serve, "serve", false, "Start notification server.")
	flag.BoolVar(&config.verbose, "verbose", false, "Verbose output.")
	flag.Parse()
	if config.serve {
		go repeater.Run()
		go handleAgent(updateChan)
		http.HandleFunc("/", handleIndex)
		http.HandleFunc("/web/", handleWeb)
		http.Handle("/ws/", websocket.Handler(handleWS))
		addr := "127.0.0.1:31415"
		log.Println("Listening on " + addr)
		log.Fatalln(http.ListenAndServe(addr, nil))
	} else {
		startClient()
	}
}

func handleIndex(resp http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	resp.Header().Add("Location", "/web/")
	resp.WriteHeader(http.StatusTemporaryRedirect)
}

func handleWeb(resp http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	resp.Write([]byte(`<!DOCTYPE html>
	<html>
	<header>
	</header>
	<body>
	<style type="text/css">
	.report {
		border: 1px solid;
	}
	.report .title {
		color: white;
		background-color: black;
	}
	.done {
		background-color: lightgray;
	}
	.line {
		display: block;
	}
	</style>
	<script type="text/javascript">
	// Set up notifications
	if (Notification.permission === "granted") {
		var notification = new Notification("Notify loaded.");
	} else if (Notification.permission !== "denied") {
		Notification.requestPermission(function (permission) {
			var notification = new Notification("Notify loaded.");
		});
	}
	// Connect to notification websocket
	var socket = new WebSocket("ws://localhost:31415/ws/");
	socket.onopen = function(event) {
		socket.send("Hello notify!");
	};
	socket.onmessage = function(event) {
		console.log("Message received: " + event.data);
		let msg = JSON.parse(event.data)
		reportCtrl = document.getElementById("report-" + msg.Id);
		if (reportCtrl === null) {
			reportCtrl = document.createElement("pre");
			reportCtrl.id = "report-" + msg.Id;
			reportCtrl.className = "report";
			reportTitle = document.createElement("div");
			reportTitle.className = "title";
			title = document.createTextNode("Report " + msg.Id);
			reportTitle.appendChild(title);
			reportCtrl.appendChild(reportTitle);
			document.body.appendChild(reportCtrl);
		}
		reportCtrl.insertAdjacentHTML('beforeend', '<div class="line">' + msg.Message + '</div>');
		reportCtrl.value = reportCtrl.value + "\n" + event.data;
		if (Notification.permission === "granted") {
			if (msg.Type === 1) {
				new Notification("Started processing ...");
			} else if (msg.Type === 2) {
				new Notification("Processing has finished.");
			}
		}
		if (msg.Type === 2) {
			reportCtrl.classList.add("done");
		}
	};
	socket.onclose = function() {
		console.log("Connection closed.");
	};
	</script>
	</body>
	</html>
	`))
}

func handleWS(ws *websocket.Conn) {
	defer ws.Close()
	// register client for updates
	updates := repeater.Register()
	defer repeater.Unregister(updates)
	// start handling messages
	for {
		msg := <-updates.Updates
		message := message{Id: msg.Id, Type: msg.Type, Message: html.EscapeString(msg.Message), Notify: msg.Notify}
		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("Failed to marshal message to JSON: %#v\n", message)
			continue
		}
		if _, err := ws.Write(data); err != nil {
			if err != io.EOF {
				log.Println("Failed to write data to websocket connection:", err.Error())
			}
			break
		}
	}
}

type message struct {
	Id      int64
	Type    MessageType
	Message string
	Notify  bool
}
