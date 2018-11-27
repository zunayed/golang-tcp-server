package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const (
	defaultHost = "0.0.0.0"
	connType    = "tcp"
	indexCmd    = "INDEX"
	removeCmd   = "REMOVE"
	queryCmd    = "QUERY"
)

type message struct {
	command      string
	pkg          string
	dependencies []string
}

type pkgServer struct {
	port               int
	address            string
	maxGoroutines      int
	sigHandlerChan     chan os.Signal
	serverShutdownChan chan struct{}
	listner            net.Listener
	dataStore          DataStore
}

func (s *pkgServer) runServer() {
	signal.Notify(s.sigHandlerChan, syscall.SIGINT, syscall.SIGTERM)
	go s.acceptConnections()

	<-s.sigHandlerChan
	s.shutdownServer()
}

func (s *pkgServer) shutdownServer() {
	close(s.serverShutdownChan)
}

func (s *pkgServer) acceptConnections() {
	guard := make(chan struct{}, s.maxGoroutines)

	for {
		select {
		case <-s.serverShutdownChan:
			break
		default:
		}

		guard <- struct{}{} // block if guard channel is already filled
		conn, err := s.listner.Accept()
		defer s.listner.Close()

		if err != nil {
			log.Errorf("Error accepting: %v\n", err)
			os.Exit(1)
		}

		go func(c net.Conn) {
			s.dataStore.handleRequest(c)
			<-guard
		}(conn)
	}
}

func newServer(port int) (*pkgServer, error) {
	address := fmt.Sprintf("%s:%d", defaultHost, port)
	listner, err := net.Listen(connType, address)
	if err != nil {
		return nil, err
	}

	log.Infof("Listening on " + listner.Addr().String())
	return &pkgServer{
		port:               port,
		address:            listner.Addr().String(),
		maxGoroutines:      100,
		sigHandlerChan:     make(chan os.Signal, 1),
		serverShutdownChan: make(chan struct{}),
		listner:            listner,
		dataStore: DataStore{
			pkgInfo:     make(map[string][]string),
			pkgRefrence: make(map[string]uint),
		},
	}, nil
}
