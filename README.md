# Hertz-SSE

(This is a community driven project)

Server-Sent events is a specification for implementing server-side-push for web frontend applications, through plain-old HTTP.
The Server-Sent Events EventSource API is standardized as part of [HTML5[1] by the W3C](https://html.spec.whatwg.org/multipage/server-sent-events.html#server-sent-events).
This repository is a fork of [manucorporat/sse](https://github.com/manucorporat/sse) for Hertz.

- [Using server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events	)
- [Stream Updates with Server-Sent Events](http://www.html5rocks.com/en/tutorials/eventsource/basics/)


## Example

```go
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/hertz-contrib/sse"

	"github.com/cloudwego/hertz/pkg/network"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

func main() {
	h := server.Default()

	h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		// client can tell server last event it received with Last-Event-ID header
		lastEventID := sse.GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// you must set status code and response headers before first render call
		c.SetStatusCode(http.StatusOK)

		sse.Stream(ctx, c, func(ctx context.Context, w network.ExtWriter) {
			// send a timestamp event to client with current time every second
			for t := range time.NewTicker(1 * time.Second).C {
				event := &sse.Event{
					Event: "timestamp",
					Data:  t.Format(time.RFC3339),
				}
				err := sse.Render(w, event)
				if err != nil {
					return
				}
			}
		})
	})

	h.Spin()
}

```


## Install
```
go get github.com/Haswf/sse
```

## Headers

The following headers are set when sse.Stream is called:
- ContentType: text/event-stream (always)
- Cache-Control: no-cache (if not set)

It's recommended to set `X-Accel-Buffering: no` if there is any proxy sitting between the server and the client.

Also see:
- [Server Sent Events are still not production ready after a decade. A lesson for me, a warning for you!](https://dev.to/miketalbot/server-sent-events-are-still-not-production-ready-after-a-decade-a-lesson-for-me-a-warning-for-you-2gie)
- [For Server-Sent Events (SSE) what Nginx proxy configuration is appropriate?](https://serverfault.com/questions/801628/for-server-sent-events-sse-what-nginx-proxy-configuration-is-appropriate)

## Chunked encoding

It's possible to send server-sent events with chunked encoding, although it is actually not [recommended](https://html.spec.whatwg.org/multipage/server-sent-events.html#authoring-notes).
> Authors are also cautioned that HTTP chunking can have unexpected negative effects on the reliability of this protocol, in particular if the chunking is done by a different layer unaware of the timing requirements. If this is a problem, chunking can be disabled for serving event streams.

To do so, you should HijackWriter with [ChunkedBodyWriter](https://www.cloudwego.io/docs/hertz/tutorials/framework-exten/response_writer/#chunkedbodywriter) before calling sse.Stream.
```go
c.Response.HijackWriter(resp.NewChunkedBodyWriter(&c.Response, c.GetWriter()))
sse.Stream(ctx, c, func(ctx context.Context, w network.ExtWriter) {
    // send event here
})
```

## Real-world examples

This repository comes with two examples to demonstrate how to build realtime applications with server sent event.

### Stock Price (examples/stockprice)
A web server that push (randomly generated) stock price periodically.
1. Run `exmaples/chat/main.go` to start server.
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


### Chat Server (examples/chat)
- an in-memory chat server that push new messages to clients using server-sent events. It supports both direct and broadcast messaging.
1. Run `exmaples/chat/main.go` to start server.
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
#

```