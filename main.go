package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
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

type dataStore struct {
	sync.RWMutex
	pkgInfo     map[string][]string
	pkgRefrence map[string]uint
}

type pkgServer struct {
	port               int
	address            string
	maxGoroutines      int
	sigHandlerChan     chan os.Signal
	serverShutdownChan chan struct{}
	listner            net.Listener
	dataStore          dataStore
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
		dataStore: dataStore{
			pkgInfo:     make(map[string][]string),
			pkgRefrence: make(map[string]uint),
		},
	}, nil
}

func (ds *dataStore) Load(key string) (value []string, ok bool) {
	ds.RLock()
	result, ok := ds.pkgInfo[key]
	ds.RUnlock()
	return result, ok
}

func (ds *dataStore) Query(key string) bool {
	ds.RLock()
	_, ok := ds.pkgInfo[key]
	ds.RUnlock()
	return ok
}

// look up packages dependencies and check if there are references
func (ds *dataStore) Remove(key string) (ok bool) {
	ds.Lock()
	defer ds.Unlock()

	deps, found := ds.pkgInfo[key]
	if !found {
		return true
	}

	delete(ds.pkgInfo, key)

	for _, dep := range deps {
		ds.pkgRefrence[dep]--
	}

	return true
}

func (ds *dataStore) increment(key string) {
	if _, ok := ds.pkgRefrence[key]; ok {
		ds.pkgRefrence[key]++
	} else {
		ds.pkgRefrence[key] = 1
	}
}

/*
	1. check if pkg and deps can be installed
	2. check if pkg already exist
		2a. if package exist remove old package and its deps
	3. store pkg and it's deps
*/
func (ds *dataStore) Store(pkg string, deps []string) (ok bool) {
	ds.Lock()
	defer ds.Unlock()

	// read all deps and find out if this can be installed
	for _, dep := range deps {
		_, ok := ds.pkgInfo[dep]
		if !ok {
			return false
		}
	}

	// check if pkg already exists and remove all old references
	existingPkgDeps, exist := ds.pkgInfo[pkg]

	if exist {
		for _, dep := range existingPkgDeps {
			ds.pkgRefrence[dep]--
		}
	}

	// store pkg and new deps
	ds.pkgInfo[pkg] = deps

	for _, dep := range deps {
		ds.increment(dep)
	}

	return true
}

func (ds *dataStore) handleMsg(incomingMsg string) (string, error) {
	m := strings.TrimSpace(incomingMsg)
	s := strings.Split(m, "|")

	if len(s) < 3 {
		return "ERROR", fmt.Errorf("Incorrect format for string %+x, %v", incomingMsg, m)
	}

	var deps = []string{}
	if s[2] != "" {
		deps = strings.Split(s[2], ",")
	}

	msg := message{
		command:      s[0],
		pkg:          s[1],
		dependencies: deps,
	}
	response := "OK"

	switch msg.command {
	case indexCmd:
		ok := ds.Store(msg.pkg, msg.dependencies)
		if !ok {
			response = "FAIL"
		}
	case removeCmd:

		ok := ds.Remove(msg.pkg)
		if !ok {
			response = "FAIL"
		}
	case queryCmd:
		ok := ds.Query(msg.pkg)
		if !ok {
			response = "FAIL"
		}
	default:
		return "ERROR", fmt.Errorf("go invalid command: %v", msg.command)
	}

	return response, nil
}

func (ds *dataStore) handleRequest(c net.Conn) {
	reader := bufio.NewReader(c)

	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			c.Close()
			return
		}

		response, err := ds.handleMsg(msg)
		if err != nil {
			log.Errorf("fail to handle message: %v %v\n", response, err)
		}

		log.Debugf("Sending response for msg\n: %v -> %v\n", msg, response)
		c.Write([]byte(response + "\n"))
	}
}

func main() {
	port := flag.Int("port", 0, "The port your server exposes to clients")
	debugMode := flag.Bool("debug", false, "Print extra info")
	flag.Parse()

	if *debugMode {
		log.Info("Debug mode active")
		log.SetLevel(log.DebugLevel)
	}

	server, _ := newServer(*port)
	server.runServer()
}
