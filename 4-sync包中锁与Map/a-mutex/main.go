package main

import (
	"fmt"
	"sync"
)

var (
	counter int
	mu      sync.Mutex
)

func inc(wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < 1000; i++ {
		mu.Lock()
		counter++
		mu.Unlock()
	}
}

func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	go inc(&wg)
	go inc(&wg)
	wg.Wait()
	fmt.Println("counter:", counter)
}
