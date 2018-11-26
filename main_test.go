package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"testing"
)

var (
	sampleMessageList = []string{
		"INDEX|c++|",
		"INDEX|vim|c++",
		"REMOVE|vim|",
		"REMOVE|python|",
		"REMOVE|lsh|",
		"INDEX|c++|",
		"INDEX|vim|c",
		"QUERY|vim|",
	}
)

func sendMessage(i int, addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	index := rand.Int31n(int32(len(sampleMessageList)))
	_, err = conn.Write([]byte(sampleMessageList[index] + "\n"))
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	fmt.Printf("response for conn %v:%v: %v", i, sampleMessageList[index], resp)
	conn.Close()
	return nil
}

func worker(addr string, jobs <-chan int, resultsCh chan<- bool) {
	for j := range jobs {
		result := true
		err := sendMessage(j, addr)
		if err != nil {
			fmt.Printf("fail: %v\n", err)
			result = false
		}

		resultsCh <- result
	}
}

func TestServer(t *testing.T) {
	jobs := make(chan int)
	results := make(chan bool)

	server, _ := newServer(0)
	go server.runServer()

	for w := 1; w <= 5; w++ {
		go worker(server.address, jobs, results)
	}

	for i := 1; i <= 1000; i++ {
		jobs <- i
		<-results
	}

	// migrate test to a struct
	close(jobs)
	server.shutdownServer()
}
