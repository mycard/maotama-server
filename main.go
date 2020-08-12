package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"golang.org/x/net/websocket"
)

var IP = os.Getenv("ADDRESS")

func transfer(local *net.UDPConn, remote *net.UDPConn, remotAddr *net.UDPAddr) {
	buffer := make([]byte, 2048)
	for {
		length, _, _ := local.ReadFromUDP(buffer)
		remote.WriteToUDP(buffer[:length], remotAddr)
	}
}
func serverTransfer(client *net.UDPConn, remoteAddr *net.UDPAddr, channel chan []byte) {
	for {
		message := <-channel
		client.WriteToUDP(message, remoteAddr)
	}
}
func listenUDP(ws *websocket.Conn) {
	server, err := net.ListenUDP("udp", nil)
	if err != nil {
		log.Println(err)
		return
	}
	clients := make(map[string]chan []byte)

	reply := fmt.Sprintf("LISTEN %s:%d", IP, server.LocalAddr().(*net.UDPAddr).Port)
	ws.Write([]byte(reply))
	for {
		message := make([]byte, 2048)
		length, clientAddr, _ := server.ReadFromUDP(message)
		channel, ok := clients[clientAddr.String()]
		if !ok {
			client, err := net.ListenUDP("udp", nil)
			if err != nil {
				log.Println(err)
				return
			}
			reply = fmt.Sprintf("CONNECT %s:%d", IP, client.LocalAddr().(*net.UDPAddr).Port)
			ws.Write([]byte(reply))
			_, remoteAddr, _ := client.ReadFromUDP(make([]byte, 2048))
			reply = fmt.Sprintf("CONNECTED %s:%d", IP, client.LocalAddr().(*net.UDPAddr).Port)
			ws.Write([]byte(reply))
			client.WriteToUDP(message[:length], remoteAddr)
			go transfer(client, server, clientAddr)
			channel := make(chan []byte)
			clients[clientAddr.String()] = channel
			go serverTransfer(client, remoteAddr, channel)
		} else {
			channel <- message[:length]
		}
	}

}
func handler(ws *websocket.Conn) {
	defer ws.Close()
	go listenUDP(ws)

	ws.Read(make([]byte, 1))
	log.Println("Websocket disconnected")

}
func main() {
	http.Handle("/", websocket.Handler(handler))
	log.Println("Maotama server started on " + IP)
	err := http.ListenAndServeTLS(":10800", "./cert/fullchain.pem",
		"./cert/privkey.pem", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
