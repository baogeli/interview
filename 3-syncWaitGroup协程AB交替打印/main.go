package main

import (
	"fmt"
	"sync"
)

func main() {

	chA := make(chan struct{})
	chB := make(chan struct{})

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 1; i <= 50; i += 2 {
			<-chA
			fmt.Println("A : ", i)
			chB <- struct{}{}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 2; i <= 50; i += 2 {
			<-chB
			fmt.Println("B : ", i)
			if i < 50 {
				chA <- struct{}{}
			}
		}
	}()

	chA <- struct{}{}
	wg.Wait()

	//chanStr := make(chan string)
	//go boa(chanStr)
	//for {
	//	fmt.Println(<-chanStr)
	//}

}
