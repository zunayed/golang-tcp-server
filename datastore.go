package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type DataStore struct {
	sync.RWMutex
	pkgInfo     map[string][]string
	pkgRefrence map[string]uint
}

func (ds *DataStore) Load(key string) (value []string, ok bool) {
	ds.RLock()
	result, ok := ds.pkgInfo[key]
	ds.RUnlock()
	return result, ok
}

func (ds *DataStore) Query(key string) bool {
	ds.RLock()
	_, ok := ds.pkgInfo[key]
	ds.RUnlock()
	return ok
}

// look up packages dependencies and check if there are references
func (ds *DataStore) Remove(key string) (ok bool) {
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

func (ds *DataStore) increment(key string) {
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
func (ds *DataStore) Store(pkg string, deps []string) (ok bool) {
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

func (ds *DataStore) handleMsg(incomingMsg string) (string, error) {
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

func (ds *DataStore) handleRequest(c net.Conn) {
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

		log.Debugf("Sending msg -> %v res -> %v", msg, response)
		c.Write([]byte(response + "\n"))
	}
}
