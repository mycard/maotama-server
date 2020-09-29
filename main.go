package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"golang.org/x/net/websocket"
)

var IP = os.Getenv("ADDRESS")

// funcs from shitama

func UDPAddrToSockAddr(addr *net.UDPAddr) []byte {

	buf := make([]byte, 8)

	binary.BigEndian.PutUint16(buf[:2], 0x200)
	binary.BigEndian.PutUint16(buf[2:4], uint16(addr.Port))
	copy(buf[4:8], addr.IP[len(addr.IP)-4:])

	return buf

}

func SockAddrToUDPAddr(buf []byte) *net.UDPAddr {

	addr := new(net.UDPAddr)
	addr.IP = make([]byte, 16)
	addr.IP[10] = 255
	addr.IP[11] = 255
	copy(addr.IP[len(addr.IP)-4:], buf[4:8])
	addr.Port = int(binary.BigEndian.Uint16(buf[2:4]))

	return addr

}

func PackData(addr *net.UDPAddr, data []byte) []byte {

	buf := make([]byte, len(data)+6)

	copy(buf[:4], addr.IP[len(addr.IP)-4:])
	binary.BigEndian.PutUint16(buf[4:6], uint16(addr.Port))
	copy(buf[6:], data)

	return buf

}

func UnpackData(buf []byte) (addr *net.UDPAddr, data []byte) {

	addr = new(net.UDPAddr)
	addr.IP = make([]byte, 16)
	addr.IP[10] = 255
	addr.IP[11] = 255
	copy(addr.IP[len(addr.IP)-4:], buf[:4])
	addr.Port = int(binary.BigEndian.Uint16(buf[4:6]))

	data = make([]byte, len(buf)-6)
	copy(data, buf[6:])
	return addr, data

}

func UDPAddrToSockAddr2(addr *net.UDPAddr, outBuf []byte) []byte {

	binary.BigEndian.PutUint16(outBuf[:2], 0x200)
	binary.BigEndian.PutUint16(outBuf[2:4], uint16(addr.Port))
	copy(outBuf[4:8], addr.IP[len(addr.IP)-4:])

	return outBuf

}

func PackData2(addr *net.UDPAddr, data []byte, outBuf []byte) []byte {

	copy(outBuf[:4], addr.IP[len(addr.IP)-4:])
	binary.BigEndian.PutUint16(outBuf[4:6], uint16(addr.Port))
	copy(outBuf[6:], data)

	return outBuf

}

func UnpackData2(buf []byte) (addr *net.UDPAddr, data []byte) {

	addr = new(net.UDPAddr)
	addr.IP = make([]byte, 16)
	addr.IP[10] = 255
	addr.IP[11] = 255
	copy(addr.IP[len(addr.IP)-4:], buf[:4])
	addr.Port = int(binary.BigEndian.Uint16(buf[4:6]))

	return addr, buf[6:]

}

func transferHostTrafficToGuest(host *net.UDPConn, guest *net.UDPConn, hostAddr *net.UDPAddr, guestAddr *net.UDPAddr) {
	buffer := make([]byte, 2048)
	for {
		length, _, _ := host.ReadFromUDP(buffer)
		guest.WriteToUDP(buffer[:length], guestAddr)
	}
}
func transferGuestTrafficToHost(host *net.UDPConn, guest *net.UDPConn, hostAddr *net.UDPAddr, guestAddr *net.UDPAddr, channel chan []byte) {
	for {
		message := <-channel
		host.WriteToUDP(message, hostAddr)
	}
}
func listenUDP(ws *websocket.Conn) {
	guest, err := net.ListenUDP("udp", nil)
	if err != nil {
		log.Println(err)
		return
	}
	guestChannelList := make(map[string]chan []byte)

	reply := fmt.Sprintf("LISTEN %s:%d", IP, guest.LocalAddr().(*net.UDPAddr).Port)
	ws.Write([]byte(reply))
	for {
		message := make([]byte, 2048)
		length, guestAddr, _ := guest.ReadFromUDP(message)
		channel, ok := guestChannelList[guestAddr.String()]
		if !ok {
			host, err := net.ListenUDP("udp", nil)
			if err != nil {
				log.Println(err)
				return
			}
			reply = fmt.Sprintf("CONNECT %s:%d", IP, host.LocalAddr().(*net.UDPAddr).Port)
			ws.Write([]byte(reply))
			_, hostAddr, _ := host.ReadFromUDP(make([]byte, 2048))
			reply = fmt.Sprintf("CONNECTED %s:%d", IP, host.LocalAddr().(*net.UDPAddr).Port)
			ws.Write([]byte(reply))
			host.WriteToUDP(message[:length], hostAddr)
			go transferHostTrafficToGuest(host, guest, hostAddr, guestAddr)
			channel := make(chan []byte)
			guestChannelList[guestAddr.String()] = channel
			go transferGuestTrafficToHost(host, guest, hostAddr, guestAddr, channel)
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
