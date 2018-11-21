package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const (
	host     = "0.0.0.0"
	connType = "tcp"
	index    = "INDEX"
	remove   = "REMOVE"
	query    = "QUERY"
)

// TODO why does this need to be capitalized to be used
type Message struct {
	command      string
	pkg          string
	dependencies []string
}

type dataStore struct {
	sync.RWMutex
	pkgInfo     map[string][]string
	pkgRefrence map[string]uint
}

func newDataStore() *dataStore {
	return &dataStore{
		pkgInfo:     make(map[string][]string),
		pkgRefrence: make(map[string]uint),
	}
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

	deps, found := ds.pkgInfo[key]
	if !found {
		ds.Unlock()
		return true
	}

	delete(ds.pkgInfo, key)

	for _, dep := range deps {
		ds.pkgRefrence[dep]--
	}

	ds.Unlock()
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

	// read all deps and find out if this can be installed

	for _, dep := range deps {
		_, ok := ds.pkgInfo[dep]
		if !ok {
			ds.Unlock()
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

	ds.Unlock()
	return true
}

func (ds *dataStore) handleMsg(message string) (string, error) {
	m := strings.TrimSpace(message)
	s := strings.Split(m, "|")

	if len(s) < 3 {
		return "ERROR", fmt.Errorf("Incorrect format for string %+x, %v", message, m)
	}

	var deps = []string{}
	if s[2] != "" {
		deps = strings.Split(s[2], ",")
	}

	msg := Message{
		command:      s[0],
		pkg:          s[1],
		dependencies: deps,
	}
	response := "OK"

	switch msg.command {
	case index:
		ok := ds.Store(msg.pkg, msg.dependencies)
		if !ok {
			response = "FAIL"
		}
	case remove:

		ok := ds.Remove(msg.pkg)
		if !ok {
			response = "FAIL"
		}
	case query:
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
			fmt.Printf("fail to handle message: %v %v\n", response, err)
		}

		fmt.Printf("Sending response for msg\n: %v -> %v\n", msg, response)
		c.Write([]byte(response + "\n"))
	}
}

func acceptConnections(l net.Listener, shutdown chan struct{}) {
	maxGoroutines := 100
	guard := make(chan struct{}, maxGoroutines)
	ds := newDataStore()

	for {
		select {
		case <-shutdown:
			break
		default:
		}

		guard <- struct{}{} // block if guard channel is already filled
		conn, err := l.Accept()
		defer l.Close()

		if err != nil {
			fmt.Printf("Error accepting: %v\n", err)
			os.Exit(1)
		}

		go func(c net.Conn) {
			ds.handleRequest(c)
			<-guard
		}(conn)
	}

}

func runServer(shutdown chan struct{}, port int) (string, error) {
	address := fmt.Sprintf("%s:%d", host, port)
	l, err := net.Listen(connType, address)
	if err != nil {
		return "", err
	}

	go acceptConnections(l, shutdown)

	return l.Addr().String(), nil
}

func main() {
	log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

	sigHandlerChan := make(chan os.Signal, 1)
	signal.Notify(sigHandlerChan, syscall.SIGINT, syscall.SIGTERM)

	port := flag.Int("port", 0, "The port your server exposes to clients")
	flag.Parse()

	serverShutdownChan := make(chan struct{})
	addr, _ := runServer(serverShutdownChan, *port)
	fmt.Println("Listening on " + addr)

	<-sigHandlerChan
	close(serverShutdownChan)
}
