package main

import (
	"fmt"
	"strings"
)

/*
todo ‌本质‌：闭包 = 函数 + 引用环境。
原本局部变量会在函数执行结束后被销毁,但闭包会保存对函数作用域的引用，并返回一个函数。这时候函数的局部变量会逃逸到堆上
*/
func createCounter() func() int {
	count := 0
	return func() int {
		count++
		return count
	}
}

func demo() func() {
	demoStr := "闭包测试"
	return func() {
		fmt.Println(demoStr)
	}
}

// 为图片文件添加后缀名
func makeSuffixFunc(suffix string) func(string) string {
	return func(personName string) string {
		if strings.HasSuffix(personName, suffix) {
			return personName
		} else {
			s := strings.Split(personName, ".")
			return s[0] + "." + suffix
		}
	}
}

func calculate(base int) (func(int) int, func(int) int) {
	add := func(i int) int {
		base += i
		return base
	}

	sub := func(i int) int {
		base -= i
		return base
	}
	return add, sub
}

func main() {
	// 闭包记数器
	//counter := createCounter()
	//fmt.Println(counter())
	//fmt.Println(counter())
	//fmt.Println(counter())

	// 匿名函数立即执行 末尾加上()
	//func() {
	//	fmt.Println("匿名函数立即执行")
	//}()

	// 匿名函数函数调用
	//hello := func() {
	//	fmt.Println("匿名函数函数调用")
	//}
	//hello()

	// 闭包简单测试
	//d := demo()
	//d()

	// 为图片文件添加后缀名
	//a := makeSuffixFunc("txt")
	//a1 := a("boa")
	//fmt.Println(a1)
	//b := makeSuffixFunc("jpg")
	//b1 := b("boa")
	//fmt.Println(b1)

	// 闭包的加减
	//add, sub := calculate(10)
	//addRes := add(1)
	//subRes := sub(5)
	//fmt.Println("base === 10, addRes === ", addRes) // addRes ===  11
	//fmt.Println("base === 10, subRes === ", subRes) // subRes ===  6
}
