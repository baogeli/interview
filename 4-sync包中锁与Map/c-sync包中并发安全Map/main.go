package main

import (
	"fmt"
	"sync"
)

func main() {
	var dict sync.Map

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			//fmt.Println("write === ", i)
			defer wg.Done()
			dict.Store(i, i)
		}(i)
	}
	wg.Wait()
	dict.Range(func(key, value interface{}) bool {
		// 1. 类型断言：因为 key/value 是 interface{}，需要转换回具体类型
		k := key.(int)
		v := value.(int)
		// 2. 业务逻辑处理
		fmt.Printf("Key: %d, Value: %d\n", k, v)
		return true
	})
}

/* 读写锁
func main() {
	var mu sync.RWMutex
	dict := make(map[int]int)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			fmt.Println("write === ", i)
			mu.Lock()
			dict[i] = i
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	fmt.Println(dict)
}
*/
