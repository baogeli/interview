package main

import (
	"log"
	"net/http"
	_ "net/http/pprof" // 关键：仅为了注册路由
	"runtime"
	"sync"
	"time"
)

func main() {
	// 打开 block / mutex 采样（默认关闭）
	// block：采样阻塞在 channel / mutex / 系统调用等的 goroutine
	runtime.SetBlockProfileRate(1) // 1 表示每次阻塞都采样；生产可设更大值降低开销
	// mutex：采样锁竞争，参数为采样比例（1/N），1 表示全部采样
	runtime.SetMutexProfileFraction(1)

	// 制造 CPU / 内存 / 阻塞 / 锁竞争 负载，方便用 pprof 观察
	busy()
	blocking()
	mutexContention()

	// 在独立的 goroutine 中启动 pprof 服务
	go func() {
		// 建议只监听本地，避免生产环境对外暴露
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// ... 你的业务 HTTP 服务在别的端口运行
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		return
	}
}

// busy 制造 CPU 与内存压力，便于 pprof 采样观察
func busy() {
	go func() {
		for {
			// 制造 CPU 压力
			_ = fib(30)
		}
	}()
	go func() {
		for {
			// 制造内存分配
			_ = make([]byte, 1<<20)
			time.Sleep(time.Millisecond)
		}
	}()
}

// blocking 制造阻塞：多个 goroutine 争抢一个无缓冲 channel
func blocking() {
	ch := make(chan struct{})
	for i := 0; i < 5; i++ {
		go func() {
			for {
				// 反复在 channel 上阻塞等待
				<-ch
			}
		}()
	}
	go func() {
		for {
			// 偶尔放一个信号，让等待者短暂被唤醒后又重新阻塞
			ch <- struct{}{}
			time.Sleep(time.Millisecond)
		}
	}()
}

// mutexContention 制造锁竞争：大量 goroutine 抢同一把锁
func mutexContention() {
	var mu sync.Mutex
	for i := 0; i < 10; i++ {
		go func() {
			for {
				mu.Lock()
				// 持锁期间做点无意义工作，放大竞争窗口
				_ = fib(10)
				mu.Unlock()
				time.Sleep(time.Millisecond)
			}
		}()
	}
}

// fib 递归计算斐波那契数，纯 CPU 计算
func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}
