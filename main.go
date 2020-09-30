package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/websocket"
)

var IP = os.Getenv("ADDRESS")

var IPinObject = net.ParseIP(IP)

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

func translateGuestToHostPackets(data *([]byte), len int, list *(map[string]*net.UDPAddr)) {
	if (*data)[0] == 0x1 {
		if len < 34 {
			return
		}
		if bytes.Compare((*data)[1:17], (*data)[17:33]) != 0 {

			src := SockAddrToUDPAddr((*data)[17:25])
			dest := (*list)[src.String()]

			if dest != nil {
				copy((*data)[17:], UDPAddrToSockAddr(dest))
				//log.Println("Translate g-to-h 0x1 packet success:", src.String(), dest.String())
			} else {
				//log.Println("Translate g-to-h 0x1 packet failed:", src.String())
			}

		}
	}
}

func translateHostToGuestPackets(data *([]byte), len int, list *(map[string]*net.UDPAddr)) {
	if (*data)[0] == 0x8 {
		len := int(binary.LittleEndian.Uint32((*data)[1:5]))

		for i := 0; i < len; i++ {

			src := SockAddrToUDPAddr((*data)[5+i*16:])
			dest := (*list)[src.String()]

			if dest != nil {
				copy((*data)[5+i*16:], UDPAddrToSockAddr(dest))
				//log.Println("Translate h-to-g 0x8 packet success:", src.String(), dest.String())
			} else {
				//log.Println("Translate h-to-g 0x8 packet failed:", src.String())
			}

		}
	}
	if (*data)[0] == 0x2 {

		src := SockAddrToUDPAddr((*data)[1:])
		dest := (*list)[src.String()]

		if dest != nil {
			copy((*data)[1:], UDPAddrToSockAddr(dest))
			//log.Println("Translate h-to-g 0x2 packet success:", src.String(), dest.String())
		} else {
			//log.Println("Translate h-to-g 0x2 packet failed:", src.String())
		}
	}
}

func transferHostTrafficToGuest(host *net.UDPConn, guest *net.UDPConn, guestAddr *net.UDPAddr, htogAddressTranslateList *(map[string]*net.UDPAddr), hostRemoteAddr *net.UDPAddr) {
	buffer := make([]byte, 2048)
	for {
		derr := host.SetReadDeadline(time.Now().Add(2 * time.Minute))
		if derr != nil {
			log.Println("Host deadline error: ", guestAddr.String(), derr)
			break
		}
		length, _, err := host.ReadFromUDP(buffer)
		if err != nil {
			log.Println("Host read error: ", guestAddr.String(), err)
			break
		}
		data := buffer[:length]
		translateHostToGuestPackets(&data, length, htogAddressTranslateList)
		guest.WriteToUDP(data, guestAddr)
	}
	(*htogAddressTranslateList)[hostRemoteAddr.String()] = nil
}
func transferGuestTrafficToHost(host *net.UDPConn, hostAddr *net.UDPAddr, guestAddr *net.UDPAddr, channel chan GuestToHostMessage, plist *(map[string]chan GuestToHostMessage), gtohAddressTranslateList *(map[string]*net.UDPAddr)) {
	for {
		exit := false
		select {
		case message := <-channel:
			if message.exit {
				exit = true
				break
			} else {
				data := message.data
				translateGuestToHostPackets(&data, message.length, gtohAddressTranslateList)
				host.WriteToUDP(data, hostAddr)
			}
		case <-time.After(time.Duration(2) * time.Minute):
			log.Println("Guest timeout: ", guestAddr.String(), hostAddr.String())
			exit = true
			break
		}
		if exit {
			break
		}
	}
	(*plist)[guestAddr.String()] = nil
	(*gtohAddressTranslateList)[guestAddr.String()] = nil
}

type GuestToHostMessage struct {
	exit   bool
	data   []byte
	length int
}

func listenUDP(ws *websocket.Conn) {
	guest, err := net.ListenUDP("udp", nil)
	if err != nil {
		log.Println("Guest listen error: ", err)
		return
	}
	guestChannelList := make(map[string]chan GuestToHostMessage)
	htogAddressTranslateList := make(map[string]*net.UDPAddr)
	gtohAddressTranslateList := make(map[string]*net.UDPAddr)

	reply := fmt.Sprintf("LISTEN %s:%d", IP, guest.LocalAddr().(*net.UDPAddr).Port)
	ws.Write([]byte(reply))
	for {
		message := make([]byte, 2048)
		derr := guest.SetReadDeadline(time.Now().Add(10 * time.Minute))
		if derr != nil {
			log.Println("Guest deadline error: ", guest.LocalAddr().(*net.UDPAddr).Port, derr)
			return
		}
		length, guestAddr, err := guest.ReadFromUDP(message)
		if err != nil {
			log.Println("Guest read error: ", guest.LocalAddr().(*net.UDPAddr).Port, err)
			return
		}
		channel, ok := guestChannelList[guestAddr.String()]
		if !ok {
			host, err := net.ListenUDP("udp", nil)
			if err != nil {
				log.Println("Host listen error: ", err)
				return
			}
			reply = fmt.Sprintf("CONNECT %s:%d", IP, host.LocalAddr().(*net.UDPAddr).Port)
			ws.Write([]byte(reply))
			derr := host.SetReadDeadline(time.Now().Add(2 * time.Minute))
			if derr != nil {
				log.Println("Knock deadline error: ", host.LocalAddr().(*net.UDPAddr).Port, derr)
				return
			}
			_, hostAddr, kerr := host.ReadFromUDP(make([]byte, 2048))
			if kerr != nil {
				log.Println("Host knock error: ", host.LocalAddr().(*net.UDPAddr).Port, kerr)
				return
			}
			reply = fmt.Sprintf("CONNECTED %s:%d", IP, host.LocalAddr().(*net.UDPAddr).Port)
			ws.Write([]byte(reply))
			host.WriteToUDP(message[:length], hostAddr)

			hostRemoteAddr := new(net.UDPAddr)
			hostRemoteAddr.IP = IPinObject
			hostRemoteAddr.Port = host.LocalAddr().(*net.UDPAddr).Port

			htogAddressTranslateList[hostRemoteAddr.String()] = guestAddr
			gtohAddressTranslateList[guestAddr.String()] = hostRemoteAddr

			//log.Println("Address map:", hostRemoteAddr.String(), guestAddr.String())

			go transferHostTrafficToGuest(host, guest, guestAddr, &htogAddressTranslateList, hostRemoteAddr)
			channel := make(chan GuestToHostMessage)
			guestChannelList[guestAddr.String()] = channel
			go transferGuestTrafficToHost(host, hostAddr, guestAddr, channel, &guestChannelList, &gtohAddressTranslateList)
		} else {
			msg := GuestToHostMessage{data: message[:length], length: length, exit: false}
			channel <- msg
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
