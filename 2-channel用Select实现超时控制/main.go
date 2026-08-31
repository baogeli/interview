package main

import (
	"context"
	"fmt"
	"time"
)

//func stopOperation() chan string {
//	// 缺点， 当主函数触发超时， go协程仍会在3s后尝试往channel中发送数据
//	// 改进思路
//	// 1. 传递context进来
//	// 2. 监听取消信号‌
//	// 3. ‌主动调用 Cancel
//	strChan := make(chan string)
//	go func() {
//		time.Sleep(3 * time.Second)
//		strChan <- "hello"
//	}()
//	return strChan
//
//}

func stopOperation(ctx context.Context) chan string {
	// 缺点， 当主函数触发超时， go协程仍会在3s后尝试往channel中发送数据
	// 改进思路
	// 1. 传递context进来
	// 2. 监听取消信号‌
	// 3. ‌主动调用 Cancel
	strChan := make(chan string)
	go func() {
		select {
		case <-time.After(5 * time.Second):
			strChan <- "hello"
		case <-ctx.Done():
			fmt.Println("goroutine receive timeout ...")
			return
		}
	}()
	return strChan

}

func main() {
	fmt.Println("start ...")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	strChan := stopOperation(ctx)
	select {
	case str := <-strChan:
		fmt.Println(str)
	case <-ctx.Done():
		fmt.Println("timeout ...")
	}
	fmt.Println("end ...")
}
