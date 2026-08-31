# WebSocket 服务端代码详解

整个服务端代码可以拆成 5 块来理解：

## 一、Client — 一个客户端连接

```go
type Client struct {
    conn *websocket.Conn
    send chan []byte
}
```

每个连接到服务的浏览器或客户端，在内存里就用一个 `Client` 表示。`conn` 是 WebSocket 连接本身，`send` 是一个发送通道（buffer 256），用于把要发给这个客户端的消息排队。

**为什么用 channel？** WebSocket 写操作是串行的。如果一个客户端同时收到 10 条消息要写，直接写会阻塞。用一个带缓冲的 channel，消息先排队，后台慢慢写出去。

## 二、Broadcast — 广播器，管理所有连接

```go
type Broadcast struct {
    mu         sync.Mutex
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
}
```

这是服务端的"大脑"。

| 字段 | 含义 |
|---|---|
| `clients` | 所有在线客户端的集合 |
| `register` | 客户端加入时通过这个 channel 通知广播器 |
| `unregister` | 客户端断开时通过这个 channel 通知广播器 |
| `broadcast` | 消息通过这个 channel 广播给所有人 |
| `mu` | 保护 `clients` map 并发安全 |

## 三、Run — 广播器的核心循环

```go
func (b *Broadcast) Run() {
    for {
        select {
        case client := <-b.register:
            // 有人加入了
        case client := <-b.unregister:
            // 有人离开了
        case message := <-b.broadcast:
            // 有新消息，发给所有客户端
        }
    }
}
```

这是一个 Go 经典模式：**单 goroutine + select 多路复用**。

1. **register**：客户端首次连接时，把 `Client` 塞进 `clients` map
2. **unregister**：客户端断开了，从 `clients` 中删除，关闭它的 send channel
3. **broadcast**：收到一条消息后，遍历所有客户端，把消息写到各自的 `send` channel

注意广播时的 `select/default`：

```go
case client.send <- message:   // 成功写入
default:                       // channel 满了，客户端太慢
    close(client.send)
    delete(b.clients, client)  // 踢掉它
```

这是一个自我保护机制——如果一个客户端读消息太慢导致 buffer 满了，直接断开它，避免拖垮整个服务。

## 四、serverHandler — 处理单个客户端

```go
func serverHandler(ws *websocket.Conn) {
    // 1. 创建 Client 对象，注册到广播器
    client := &Client{conn: ws, send: make(chan []byte, 256)}
    broadcast.register <- client

    // 2. 启动一个 goroutine 读消息
    go func() {
        for {
            _, msg, err := ws.ReadMessage()
            if err != nil {
                broadcast.unregister <- client  // 异常断开，注销
                return
            }
            broadcast.broadcast <- msg   // 把消息丢给广播器
        }
    }()

    // 3. 当前 goroutine 负责写消息给这个客户端
    for {
        msg, ok := <-client.send
        if !ok { return }  // send channel 关了，退出
        ws.WriteMessage(websocket.TextMessage, msg)
    }
}
```

这里有一个关键的设计：**读和写分离到两个 goroutine**。

- **读 goroutine**：不断读取这个客户端发来的消息，丢进广播器，由广播器转发给其他人
- **写 goroutine**（当前函数）：不断从 `client.send` channel 取消息，写回给这个客户端

为什么不能混在一起？因为 WebSocket 连接一次只能写一条消息，读和写如果交替进行会导致消息交叉、时序错乱。

## 五、main — 启动入口

```go
func main() {
    broadcast = NewBroadcast()
    go broadcast.Run()     // 启动广播器（单 goroutine）

    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, _ := upgrader.Upgrade(w, r, nil)  // HTTP → WebSocket
        serverHandler(conn)                       // 处理这个连接
    })

    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

流程就是：
1. 启动广播器 goroutine
2. 注册 `/ws` 路由
3. 等 HTTP 请求进来 → 通过 `upgrader.Upgrade` 把普通 HTTP 连接升级为 WebSocket → 交给 `serverHandler`

**Upgrade** 的本质就是：HTTP 连接上双方先协商（握手），然后协议从 HTTP 变成 WebSocket，之后就是全双工的消息通道了。

## 整体数据流

```
客户端A ──发消息──> 广播器 ──发消息──> 客户端B
客户端B ──发消息──> 广播器 ──发消息──> 客户端A
```

每条消息的走向：
1. 客户端A 通过 WebSocket 发消息
2. `serverHandler` 的读 goroutine 收到，丢进 `broadcast.broadcast` channel
3. `Broadcast.Run()` 的 select 捕获到消息，遍历所有客户端写入各自的 `send` channel
4. 每个客户端自己的写 goroutine 从 `send` channel 取消息，通过 WebSocket 写回去

整个设计用 Go 原生的 goroutine + channel 实现，不需要外部数据库或消息队列，纯内存运行。
