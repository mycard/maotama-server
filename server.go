package main

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
)

const (
	ALLOCATE          = 0x003
	REFRESH           = 0x004
	SEND              = 0x006
	DATA              = 0x007
	CREATE_PERMISSION = 0x008
	CHANNEL_BIND      = 0x009
)

const (
	MAGIC_COOKIE = 0x2112A442
	HEADER_LEN   = 20
)

func handle(msg []byte) {
	header := msg[:20]
	method, class, magic, transacation, err := parseHeader(header)

}
func parseMessage(body []byte) (int16, int16, []byte, error) {
	if len(body) < 2 {
		return errors.New("No more attributes in message body")
	}
	t := binary.BigEndian.Uint16(body[:2])
	length := binary.BigEndian.Uint16(body[2:4])
	return t, length, body[4 : length+4], nil
}
func parseHeader(header []byte) (int16, int8, int, []byte, error) {
	if len(header) != HEADER_LEN {
		return 0, 0, 0, nil, errors.New("header length not correct")
	}
	method := int16(header[:2] & 0xFEEF)
	class := int8(header[:2] & 0x0110)
	length := binary.BigEndian.Uint16(header[2:4])
	magic := binary.BigEndian.Uint32(header[4:8])
	if magic != MAGIC_COOKIE {
		return 0, 0, 0, nil, errors.New("Magic Cookie is invalid")
	}
	transacation := header[8:]
	return method, class, length, magic, transacation, nil

}
func initUDPServer(addr string) *net.UDPConn {
	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		log.Fatalf("Can't resolve address %s\n", addr)
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		log.Fatalf("Listen error on address %s,reason %v\n", addr, err)
	}
	return conn
}
