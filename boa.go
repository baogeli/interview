package main

import (
	"fmt"
	"time"
)

func boa(chanStr chan string) {
	localTimeTags := "2006-01-02 15:04:05"
	defer func() {
		fmt.Println("boa exit")
		close(chanStr)
	}()
	for i := 0; i < 5; i++ {
		chanStr <- time.Now().Format(localTimeTags)
		time.Sleep(1 * time.Second)
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("boa panic:", r)
		}
	}()
	chanStr := make(chan string)
	go boa(chanStr)
	for {
		if str, ok := <-chanStr; ok {
			fmt.Println(str)
		} else {
			break
		}
	}
}
