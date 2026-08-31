package main

type SliceHeader struct {
	Data uintptr // 指向底层数组第一个元素的指针
	Len  int     // 当前 slice 的长度（可访问的元素个数）
	Cap  int     // 底层数组从 Data 起始的可用容量（Len 到 Cap 之间可 append 但不暴露）
}

func main() {
	/*
		关键点
		1. slice 本身只是个"描述符"，不持有数据
		slice 变量通常 24 字节（指针 8 + Len 8 + Cap 8，64 位平台）。真正的数据在堆上的底层数组里。多个 slice 可以共享同一个底层数组。

		2. 共享底层数组的副作用
		a := make([]int, 3, 5)
		b := a[1:3]
		b[0] = 99
		// a[1] 也会变成 99 —— 因为 a 和 b 指向同一数组
		b 的 Data 指向 a 的第二个元素，Len=2, Cap=4。

		3. append 与扩容
		- len < cap：append 直接写入底层数组剩余空间，共享数组的其他 slice 会受影响。
		- len == cap：分配一块更大的新数组（通常按 1→2 倍策略，大 slice 增长因子趋近于 1.25），拷贝原数据，原 slice 与新 slice 不再共享。

		4. 为什么传 slice 到函数改内容要小心
		- 传 slice 是值传递，但拷贝的是 header（包括指针），所以函数内通过下标改元素会影响原数组。
		- 但若函数内 append 导致扩容，函数内的 slice 会指向新数组，原 slice 不受影响——如果要把扩容结果带回，需要返回 slice 或传 *[]T。

		5. 零值
		var s []int 的 header 是 {Data:nil, Len:0, Cap:0}，与 make([]int,0) / []int{} 等价但 nil 语义不同（前者 s == nil 为 true，后者为 false）。
	*/
}
