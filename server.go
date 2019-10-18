package main

import (
	"fmt"
	"net"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func handleMessage(conn *net.UDPConn, data []byte, remote *net.UDPAddr) {
	//fmt.Println("Received:", string(data), "from", remote)
	_, err := conn.WriteToUDP(data, remote)
	check(err)
	//fmt.Println("Sent", n, "bytes", data, "to", remote)
}

func main() {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 25565,
	})
	check(err)
	defer conn.Close()
	fmt.Printf("server listening on %s\n", conn.LocalAddr().String())

	for {
		buffer := make([]byte, 32)
		receivedLen, remote, err := conn.ReadFromUDP(buffer)
		check(err)
		go handleMessage(conn, buffer[:receivedLen], remote)
	}
}
