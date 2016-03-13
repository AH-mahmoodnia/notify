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
	webAddr := "127.0.0.1:31415"
	agentAddr := "127.0.0.1:31416"
	if config.serve {
		go repeater.Run()
		go handleAgent(agentAddr, updateChan)
		http.HandleFunc("/", handleIndex)
		http.HandleFunc("/web/", handleWeb)
		http.Handle("/ws/", websocket.Handler(handleWS))
		log.Println("Listening for clients on " + webAddr)
		log.Fatalln(http.ListenAndServe(webAddr, nil))
	} else {
		startClient(agentAddr)
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
	<head>
	<style type="text/css">
	h1 {
		color: #333;
		font-size: 14pt;
		text-decoration: underline;
	}
	.report {
		border: 1px solid;
	}
	.report .title {
		color: white;
		background-color: black;
		border: 0px 0px 0px 1px solid black;
	}
	.report .title a {
		color: black;
		background-color: white;
	}
	.done {
		background-color: lightgray;
	}
	.line {
		display: block;
	}
	</style>
	</head>
	<body>
	<h1>Notify</h1>
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
			reportCtrl = createReport(msg.Id);
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

	function createReport(id) {
		reportCtrl = document.createElement("pre");
		reportCtrl.id = "report-" + id;
		reportCtrl.className = "report";
		removeLink = document.createElement("a");
		removeLink.onclick = function() {
			if (window.confirm("Remove this report?")) {
				reportCtrl.remove();
			}
		};
		removeLinkText = document.createTextNode(" X ");
		removeLink.appendChild(removeLinkText);
		reportTitle = document.createElement("div");
		reportTitle.className = "title";
		reportTitle.appendChild(removeLink);
		reportTitle.appendChild(document.createTextNode(" "));
		title = document.createTextNode('Report ' + id);
		reportTitle.appendChild(title);
		reportCtrl.appendChild(reportTitle);
		return reportCtrl;
	}
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
