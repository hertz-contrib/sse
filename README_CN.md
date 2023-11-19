# Hertz-SSE

（这是一个社区驱动的项目）

[English](README.md) ｜ 中文

服务端发送事件（Server-Sent events）是一种通过简单的基于 HTTP 请求的方式为 Web 前端应用程序实现服务器端推送的规范 。 服务端发送的事件 EventSource API 已标准化为一部分 [W3C 的 HTML5[1]](https://html.spec.whatwg.org/multipage/server-sent-events.html#server-sent-events)。

该存储库是 Hertz 的 [manucorporat/sse](https://github.com/manucorporat/sse) 和 [r3labs/sse](https://github.com/r3labs/sse/) 的分支。

- [使用服务端发送事件](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events    )
- [使用服务端发送事件进行流更新](http://www.html5rocks.com/en/tutorials/eventsource/basics/)

## 安装

```
go get github.com/hertz-contrib/sse
```

## 示例

### 服务端

```go
package main

import (
  "context"
  "net/http"
  "time"

  "github.com/hertz-contrib/sse"

  "github.com/cloudwego/hertz/pkg/app"
  "github.com/cloudwego/hertz/pkg/app/server"
  "github.com/cloudwego/hertz/pkg/common/hlog"
)

func main() {
  h := server.Default()

  h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
    // 客户端可以通过 Last-Event-ID 标头告诉服务端它收到的最后一个事件
    lastEventID := sse.GetLastEventID(c)
    hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

    // 必须在第一次调用之前设置状态代码和响应标头
    c.SetStatusCode(http.StatusOK)
    s := sse.NewStream(c)
    for t := range time.NewTicker(1 * time.Second).C {
      event := &sse.Event{
        Event: "timestamp",
        Data:  []byte(t.Format(time.RFC3339)),
      }
      err := s.Publish(event)
      if err != nil {
        return
      }
    }
  })

  h.Spin()
}

```

### 客户端

```go
package main

import (
	"context"
	"sync"

	"github.com/hertz-contrib/sse"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

var wg sync.WaitGroup

func main() {
  wg.Add(2)	
  go func() {
    // 传入 server 端 URL 初始化客户端  	  
    c := sse.NewClient("http://127.0.0.1:8888/sse")

    // 连接到服务端的时候触发
    c.OnConnect(func(ctx context.Context, client *sse.Client) {
      hlog.Infof("client1 connect to server %s success with %s method", c.URL, c.Method)
    })

    // 服务端断开连接的时候触发
    c.OnDisconnect(func(ctx context.Context, client *sse.Client) {
      hlog.Infof("client1 disconnect to server %s success with %s method", c.URL, c.Method)
    })

    events := make(chan *sse.Event)
    errChan := make(chan error)
    go func() {
      cErr := c.Subscribe(func(msg *sse.Event) {
        if msg.Data != nil {
          events <- msg
          return
        }
      })
      errChan <- cErr
    }()
    for {
      select {
      case e := <-events:
        hlog.Info(e)
      case err := <-errChan:
        hlog.CtxErrorf(context.Background(), "err = %s", err.Error())
		wg.Done()
        return
      }
    }
  }()

  go func() {
    // 传入 server 端 URL 初始化客户端  
    c := sse.NewClient("http://127.0.0.1:8888/sse")

    // 连接到服务端的时候触发
    c.OnConnect(func(ctx context.Context, client *sse.Client) {
      hlog.Infof("client2 %s connect to server success with %s method", c.URL, c.Method)
    })

    // 服务端断开连接的时候触发
    c.OnDisconnect(func(ctx context.Context, client *sse.Client) {
      hlog.Infof("client2 %s disconnect to server success with %s method", c.URL, c.Method)
    })

    events := make(chan *sse.Event)
    errChan := make(chan error)
    go func() {
      cErr := c.Subscribe(func(msg *sse.Event) {
        if msg.Data != nil {
          events <- msg
          return
        }
      })
      errChan <- cErr
    }()
    for {
      select {
      case e := <-events:
        hlog.Info(e)
      case err := <-errChan:
        hlog.CtxErrorf(context.Background(), "err = %s", err.Error())
		wg.Done()
        return
      }
    }
  }()

  select {}
}

```

## 真实场景示例

该仓库附带两个 server 示例，演示如何使用 SSE 构建服务端真实场景应用程序

### 股票价格 (examples/server/stockprice)

定期推送（随机生成）股票价格的网络服务器

1. 运行 `examples/server/chat/main.go` 来开启服务
2. 发送 GET 请求到 `/price`

```bash
curl -N --location 'localhost:8888/price'
#id:1681141432283
#event:AAPL
#data:92.607347
#
#id:1681141432283
#event:AMZN
#data:73.540894
#
#id:1681141433283
#event:AAPL
#data:23.536702
#
#id:1681141433283
#event:AMZN
#data:63.156229
#

```

### 聊天服务 (examples/server/chat)

使用服务器发送的事件将新消息推送到客户端的聊天服务器。 支持直播和广播两种消息传递方式

1. 运行 `exmaples/server/chat/main.go` 来开启服务
2. 发送 GET 请求到 `/chat/sse`

```bash
# 代表用户 hertz 接收消息
curl -N --location 'http://localhost:8888/chat/sse?username=hertz'
```

3. 打开一个新的终端然后发送消息给 hertz

```bash
# 发送一个广播消息
curl --location --request POST 'http://localhost:8888/chat/broadcast?from=kitex&message=cloudwego'
# 直接发送一个消息
curl --location --request POST 'http://localhost:8888/chat/direct?from=kitex&message=hello%20hertz&to=hertz'
```

在第一个终端你能看到两条消息

```bash
curl -N --location 'http://localhost:8888/chat/sse?username=hertz'
#event:broadcast
#data:{"Type":"broadcast","From":"kitex","To":"","Message":"cloudwego","Timestamp":"2023-04-10T23:48:55.019742+08:00"}
#
#event:direct
#data:{"Type":"direct","From":"kitex","To":"hertz","Message":"hello hertz","Timestamp":"2023-04-10T23:48:56.212855+08:00"}
#

```