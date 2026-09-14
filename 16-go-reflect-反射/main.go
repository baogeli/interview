package main

import (
	"fmt"
	"reflect"
)

// 定义 程序在运行时检查自身结构的能力

// 核心 interface{} 和 reflect包

func main() {
	// 1. Type 类型信息
	var i int32 = 43
	typeOf := reflect.TypeOf(i)
	// 2. Value 值信息
	valueOf := reflect.ValueOf(i)
	fmt.Println("Type:", typeOf)
	fmt.Println("Value:", valueOf)
}
