# Hertz-SSE

**Notice**: This project is now archived. For users and those seeking similar functionality, please use Hertz's built-in SSE implementation for a better user experience. Check documentation [SSE](https://www.cloudwego.io/docs/hertz/tutorials/basic-feature/sse/).

(This is a community driven project)

English ｜ [中文](README_CN.md)

Server-Sent events is a specification for implementing server-side-push for web frontend applications, through plain-old
HTTP.
The Server-Sent Events EventSource API is standardized as part
of [HTML5[1] by the W3C](https://html.spec.whatwg.org/multipage/server-sent-events.html#server-sent-events).
This repository is a fork of [manucorporat/sse](https://github.com/manucorporat/sse) and [r3labs/sse](https://github.com/r3labs/sse/) for Hertz.

- [Using server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events    )
- [Stream Updates with Server-Sent Events](http://www.html5rocks.com/en/tutorials/eventsource/basics/)

## Install

```
go get github.com/hertz-contrib/sse
```

## Example

### Server

see: [examples/server/quickstart/main.go](examples/server/quickstart/main.go)

```go
package main

import (
  "context"
  "net/http"
  "time"

  "github.com/cloudwego/hertz/pkg/app"
  "github.com/cloudwego/hertz/pkg/app/server"
  "github.com/cloudwego/hertz/pkg/common/hlog"

  "github.com/hertz-contrib/sse"
)

func main() {
  h := server.Default()

  h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
    // client can tell server last event it received with Last-Event-ID header
    lastEventID := sse.GetLastEventID(c)
    hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

    // you must set status code and response headers before first render call
    c.SetStatusCode(http.StatusOK)
    s := sse.NewStream(c)

    count := 0
    sendCountLimit := 10
    for t := range time.NewTicker(1 * time.Second).C {
      event := &sse.Event{
        Event: "timestamp",
        Data:  []byte(t.Format(time.RFC3339)),
      }
      err := s.Publish(event)
      if err != nil {
        return
      }
      count++
      if count >= sendCountLimit {
        // send end flag to client
        err := s.Publish(&sse.Event{
          Event: "end",
          Data:  []byte("end flag"),
        })
        if err != nil {
          return
        }
        break
      }
    }
  })

  h.Spin()
}
```

### Client

see: [examples/client/quickstart/main.go](examples/client/quickstart/main.go)

```go
package main

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol"

	"github.com/hertz-contrib/sse"
)

var wg sync.WaitGroup

func main() {
	wg.Add(2)
	go func() {
		// create Hertz client 
		hCli, err := client.NewClient()
		if err != nil {
			hlog.Errorf("create Hertz client failed, err: %v", err)
			return
		}
		// inject Hertz client to create SSE client
		c, err := sse.NewClientWithOptions(sse.WithHertzClient(hCli))
		if err != nil {
			hlog.Errorf("create SSE client failed, err: %v", err)
			return
		}

		// touch off when connected to the server
		c.SetOnConnectCallback(func(ctx context.Context, client *sse.Client) {
			hlog.Infof("client1 connect to server success")
		})

		// touch off when the connection is shutdown
		c.SetDisconnectCallback(func(ctx context.Context, client *sse.Client) {
			hlog.Infof("client1 disconnect to server success")
		})

		events := make(chan *sse.Event)
		errChan := make(chan error)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// build the req sent with each SSE request	
			req := &protocol.Request{}
			req.SetRequestURI("http://127.0.0.1:8888/sse")
			cErr := c.SubscribeWithContext(ctx, func(msg *sse.Event) {
				if msg.Data != nil {
					events <- msg
					return
				}
			}, sse.WithRequest(req))
			errChan <- cErr
		}()
		go func() {
			time.Sleep(5 * time.Second)
			cancel()
			hlog.Info("client1 subscribe cancel")
		}()
		for {
			select {
			case e := <-events:
				hlog.Infof("client1, %+v", e)
			case err := <-errChan:
				if err == nil {
					hlog.Info("client1, ctx done, read stop")
				} else {
					hlog.CtxErrorf(ctx, "client1, err = %s", err.Error())
				}
				wg.Done()
				return
			}
		}
	}()

	go func() {
		// create Hertz client 
		hCli, err := client.NewClient()
		if err != nil {
			hlog.Errorf("create Hertz client failed, err: %v", err)
			return
		}
		// inject Hertz client to create SSE client
		c, err := sse.NewClientWithOptions(sse.WithHertzClient(hCli))
		if err != nil {
			hlog.Errorf("create SSE client failed, err: %v", err)
			return
		}

		// touch off when connected to the server
		c.SetOnConnectCallback(func(ctx context.Context, client *sse.Client) {
			hlog.Infof("client2 connect to server success")
		})

		// touch off when the connection is shutdown
		c.SetDisconnectCallback(func(ctx context.Context, client *sse.Client) {
			hlog.Infof("client2 disconnect to server success")
		})

		events := make(chan *sse.Event, 10)
		errChan := make(chan error)
		go func() {
			// build the req sent with each SSE request	
			req := &protocol.Request{}
			req.SetRequestURI("http://127.0.0.1:8888/sse")
			cErr := c.Subscribe(func(msg *sse.Event) {
				if msg.Data != nil {
					events <- msg
					return
				}
			}, sse.WithRequest(req))
			errChan <- cErr
		}()

		streamClosed := false
		for {
			select {
			case e := <-events:
				hlog.Infof("client2, %+v", e)
				time.Sleep(2 * time.Second) // do something blocked
				// When the event ends, you should break out of the loop.
				if checkEventEnd(e) {
					wg.Done()
					return
				}
			case err := <-errChan:
				if err == nil {
					// err is nil means read io.EOF, stream is closed
					streamClosed = true
					hlog.Info("client2, stream closed")
					// continue read channel events
					continue
				}
				hlog.CtxErrorf(context.Background(), "client2, err = %s", err.Error())
				wg.Done()
				return
			default:
				if streamClosed {
					hlog.Info("client2, events is empty and stream closed")
					wg.Done()
					return
				}
			}
		}
	}()

	wg.Wait()
}

func checkEventEnd(e *sse.Event) bool {
	// check e.Data or e.Event. It depends on the definition of the server
	return e.Event == "end" || string(e.Data) == "end flag"
}

```

## Real-world examples

This repository comes with two server-examples to demonstrate how to build realtime applications with server-sent event.

### Stock Price (examples/server/stockprice)

A web server that push (randomly generated) stock price periodically.

1. Run `exmaples/server/chat/main.go` to start server.
2. Send a GET request to `/price`

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

### Chat Server (examples/server/chat)

A chat server that push new messages to clients using server-sent events. It supports both direct and broadcast
  messaging.

1. Run `examples/server/chat/main.go` to start server.
2. Send a get request to `/chat/sse`.

```bash
# receive message on behalf of user hertz
curl -N --location 'http://localhost:8888/chat/sse?username=hertz'
```

3. Open a new terminal and send messages to hertz.

```bash
# send a broadcast message
curl --location --request POST 'http://localhost:8888/chat/broadcast?from=kitex&message=cloudwego'
# send a direct message
curl --location --request POST 'http://localhost:8888/chat/direct?from=kitex&message=hello%20hertz&to=hertz'
```

On the first terminal, you should see 2 messages.

```bash
curl -N --location 'http://localhost:8888/chat/sse?username=hertz'
#event:broadcast
#data:{"Type":"broadcast","From":"kitex","To":"","Message":"cloudwego","Timestamp":"2023-04-10T23:48:55.019742+08:00"}
#
#event:direct
#data:{"Type":"direct","From":"kitex","To":"hertz","Message":"hello hertz","Timestamp":"2023-04-10T23:48:56.212855+08:00"}

```

## Benchmark Results

All benchmarks are stored for each commit, they can be viewed here:

https://hertz-contrib.github.io/sse/benchmarks/
