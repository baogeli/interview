package main

import (
	"fmt"
	"sync"
	"time"
)

// 场景: 假设有一个全局配置 Map，频繁被读取，偶尔被更新

var (
	config map[string]string
	rwMu   sync.RWMutex // 定义读写锁
)

func init() {
	config = make(map[string]string)
}

// 读操作：使用 RLock，支持高并发读取
func getConfig(key string) string {
	//rwMu.RLock()         // 1. 加读锁
	//defer rwMu.RUnlock() // 2. 确保释放读锁
	return config[key]
}

// 写操作：使用 Lock，独占写入
func setConfig(key, value string) {
	rwMu.Lock()         // 1. 加写锁（阻塞其他读写）
	defer rwMu.Unlock() // 2. 确保释放写锁
	config[key] = value
	fmt.Printf("配置已更新: %s = %s\n", key, value)
}

func main() {
	var wg sync.WaitGroup
	// 启动 5 个协程持续读取
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				val := getConfig("app_name")
				fmt.Printf("协程 %d 读取到: %s\n", id, val)
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}

	// 启动 1 个协程间歇性写入
	wg.Add(1)
	go func() {
		defer wg.Done()
		setConfig("app_name", "MyApp_v1")
		time.Sleep(200 * time.Millisecond)
		setConfig("app_name", "MyApp_v2")
	}()
	wg.Wait()
}
