package main

import (
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"log"
	"net"
	"os"
	"sync"
)

const (
	handshake = iota + 1
	handshakeResponse
	position
	positionResponse
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type coords struct {
	x, y uint8
}

type state struct {
	*sync.Mutex
	positions map[uint8]coords
}

type server struct {
	conn  *net.UDPConn
	state *state
	log   logr.Logger
}

func (s *state) setPosition(id, x, y uint8) {
	s.Lock()
	s.positions[id] = coords{x, y}
	s.Unlock()
}

type message struct {
	opcode uint8
	data   []byte
}

func (m message) encode() []byte {
	payload := make([]byte, 0, len(m.data)+1) // +1 for opcode
	payload = append(payload, m.opcode)
	payload = append(payload, m.data...)
	return payload
}

func decode(payload []byte) (message, error) {
	if len(payload) < 2 {
		return message{}, errors.New("payload too short")
	}
	return message{
		opcode: payload[0],
		data:   payload[1:],
	}, nil
}

func (s *server) handleHandshake(m message, remote *net.UDPAddr) {
	s.log.V(5).Info("opcode = handshake", "data", fmt.Sprintf("%x", m.data))
	playerId := m.data[0]
	_, present := s.state.positions[playerId]
	if present {
		s.log.Error(nil, "player is already present", "player ID", playerId)
		return
	}
	s.state.setPosition(playerId, 0, 0)
	response := message{handshakeResponse, []byte{playerId}}
	_, err := s.conn.WriteToUDP(response.encode(), remote)
	if err != nil {
		s.log.Error(err, "response failed")
		return
	}
	s.log.V(5).Info("handshake response sent", "data", fmt.Sprintf("%x", response.data), "to", remote)
}

func (s *server) handlePosition(m message, remote *net.UDPAddr) {
	s.log.V(5).Info("opcode = position", "data", fmt.Sprintf("%x", m.data))
	if len(m.data) < 3 {
		s.log.Error(nil, "data too short in position packet", "opcode", fmt.Sprintf("%x", m.opcode), "data", fmt.Sprintf("%x", m.data))
		return
	}
	playerId := m.data[0]
	x := m.data[1]
	y := m.data[2]
	s.state.setPosition(playerId, x, y)
	positionsData := make([]byte, 0, len(s.state.positions))
	for k, v := range s.state.positions {
		if k != playerId {
			positionsData = append(positionsData, k, v.x, v.y)
			s.log.V(10).Info("position data", "player", k, "x", v.x, "y", v.y)
		}
	}
	response := message{handshakeResponse, positionsData}
	_, err := s.conn.WriteToUDP(response.encode(), remote)
	if err != nil {
		s.log.Error(err, "response failed")
		return
	}
	s.log.V(5).Info("response sent", "data", fmt.Sprintf("%x", response.data), "to", remote)
}

func (s *server) handle(m message, remote *net.UDPAddr) {
	s.log.V(5).Info("received message", "opcode", fmt.Sprintf("%x", m.opcode), "from", remote)
	switch m.opcode {
	case handshake:
		s.handleHandshake(m, remote)
	case position:
		s.handlePosition(m, remote)
	default:
		s.log.Error(nil, "unknown opcode")
	}
}

func main() {
	stdr.SetVerbosity(0)
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 25565,
	})
	check(err)
	defer conn.Close()
	server := server{
		conn: conn,
		state: &state{
			Mutex:     &sync.Mutex{},
			positions: make(map[uint8]coords),
		},
		log: stdr.New(log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)),
	}
	server.log.V(0).Info("server listening", "address", conn.LocalAddr().String())
	for {
		buffer := make([]byte, 32)
		receivedLen, remote, err := conn.ReadFromUDP(buffer)
		if err != nil {
			server.log.Error(err, "UDP packet dropped due to error")
			continue
		}
		m, err := decode(buffer[:receivedLen])
		if err != nil {
			server.log.Error(err, "UDP packet dropped due to error")
			continue
		}
		go server.handle(m, remote)
	}
}
