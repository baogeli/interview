package main

import (
	"context"
	"fmt"
	"time"
)

// 在异步场景中，控制goroutine的生命周期

/*
4个interface

	type Context interface {
		Deadline() (deadline time.Time, ok bool)
		Done() <-chan struct{}
		Err() error
		Value(key interface{}) interface{}
	}
*/
func main() {

	ctx, cancelFunc := context.WithCancel(context.Background())
	done := ctx.Done()
	go func() {
		fmt.Println("3秒后执行cancel操作....")
		time.Sleep(3 * time.Second)
		cancelFunc()
	}()

	fmt.Println("等待channel ...")
	<-done
	fmt.Println("channel接收到数据", ctx.Err())

}
