package main

import "fmt"

// todo recover捕获 panic b func, 使 c func 继续执行
func main() {
	func() {
		fmt.Println("a func")
	}()

	func() {
		defer func() {
			panicMsg := recover()
			if panicMsg != nil {
				fmt.Println("panic b func")
			}
		}()
		panic("panic b func")
	}()

	func() {
		fmt.Println("c func")
	}()
}
