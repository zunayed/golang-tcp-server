package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"testing"
)

var (
	message = []string{
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
	defer conn.Close()

	index := rand.Int31n(int32(len(message)))
	_, err = conn.Write([]byte(message[index] + "\n"))
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	fmt.Printf("response for conn %v:%v: %v", i, message[index], resp)
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
	serverShutdownChan := make(chan struct{})
	jobs := make(chan int)
	results := make(chan bool)

	addr, _ := runServer(serverShutdownChan, 0)
	fmt.Println("Listening on " + addr)

	for w := 1; w <= 5; w++ {
		go worker(addr, jobs, results)
	}

	for i := 1; i <= 1000; i++ {
		jobs <- i
	}

	close(jobs)
	close(serverShutdownChan)
}
