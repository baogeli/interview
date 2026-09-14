# pprof 调试与使用指南

> 配套 `main.go`：通过 `_ "net/http/pprof"` 空导入注册路由，pprof 服务跑在 `localhost:6060`，业务服务跑在 `:8080`。

## 当前 main.go 做了什么

```go
import _ "net/http/pprof"  // 空导入，只为触发它的 init()，把路由注册到默认 mux
```

`net/http/pprof` 在 `init()` 里往 `http.DefaultServeMux` 注册了一组调试路由。单独起一个 goroutine 在 `localhost:6060` 跑 pprof 服务，业务服务跑在 `:8080`，两者互不干扰，且 pprof 只监听本地避免外泄。

## 可用的调试端点

启动后访问 `http://localhost:6060/debug/pprof/` 会看到导航页，关键端点：

| 端点 | 用途 |
|------|------|
| `/debug/pprof/profile` | CPU profile，默认采样 30s |
| `/debug/pprof/heap` | 堆内存分配（对象/字节，可切换） |
| `/debug/pprof/goroutine` | 当前所有 goroutine 栈 |
| `/debug/pprof/block` | 阻塞在同步原语上的信息 |
| `/debug/pprof/mutex` | 锁竞争 |
| `/debug/pprof/trace` | 执行轨迹（5s） |

> 注意：goroutine/heap 等默认开启；**block 和 mutex 需要在代码里显式调用 `runtime.SetBlockProfileRate` / `runtime.SetMutexProfileFraction` 才会采集**。

## 怎么真正调试（命令行）

用 `go tool pprof` 连上去分析。下面以 CPU 和内存为例：

```bash
# 1. 编译并运行你的程序（确保 6060 在监听）

# 2. CPU 分析，采样 30s
go tool pprof http://localhost:6060/debug/pprof/profile
# 进入交互界面后常用命令：
#   top10      看耗时最高的 10 个函数
#   list 函数名  看某个函数的逐行耗时
#   web        生成调用图（需 graphviz）
#   png        导出图片

# 3. 内存分析
go tool pprof http://localhost:6060/debug/pprof/heap
#   top        看内存占用最高的
#   inuse_space / alloc_space  切换"在用"还是"累计分配"
```

也可以先下载成文件再分析（适合生产环境）：

```bash
curl -o cpu.prof http://localhost:6060/debug/pprof/profile?seconds=30
go tool pprof cpu.prof
```

## 可跑的演示代码（制造负载）

当前 `main.go` 没有业务逻辑，pprof 上去也没东西可看。可补一段能产生 CPU/内存压力的代码：

```go
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
```
