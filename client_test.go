/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sse

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

var mldata = `{
	"key": "value",
	"array": [
		1,
		2,
		3
	]
}`

func newServer(empty bool, port string) {
	h := server.Default(server.WithHostPorts("0.0.0.0:" + port))

	h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		// client can tell server last event it received with Last-Event-ID header
		lastEventID := GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// you must set status code and response headers before first render call
		c.SetStatusCode(http.StatusOK)
		s := NewStream(c)
		publishMsgs(s, empty, 1000)
	})
	h.Run()
}

func newServerOnConnect(empty bool, port string) {
	h := server.Default(server.WithHostPorts("0.0.0.0:" + port))

	h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		// client can tell server last event it received with Last-Event-ID header
		lastEventID := GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// you must set status code and response headers before first render call
		c.SetStatusCode(http.StatusOK)
		s := NewStream(c)
		publishMsgs(s, empty, 10000)
	})
	h.Run()
}

func newServerBigData(data []byte, port string) {
	h := server.Default(server.WithHostPorts("0.0.0.0:" + port))

	h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		// client can tell server last event it received with Last-Event-ID header
		lastEventID := GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// you must set status code and response headers before first render call
		c.SetStatusCode(http.StatusOK)
		s := NewStream(c)
		err := s.Publish(&Event{Data: data})
		if err != nil {
			return
		}
	})

	h.Spin()
}

func newMultilineServer(port string) {
	h := server.Default(server.WithHostPorts("0.0.0.0:" + port))

	h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		// client can tell server last event it received with Last-Event-ID header
		lastEventID := GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// you must set status code and response headers before first render call
		c.SetStatusCode(http.StatusOK)
		s := NewStream(c)
		publishMultilineMessages(s, 100000)
	})

	h.Spin()
}

func newServer401(port string) {
	h := server.Default(server.WithHostPorts("0.0.0.0:" + port))

	h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		c.SetStatusCode(http.StatusUnauthorized)
	})

	h.Spin()
}

func newServerWithPOSTBody(empty bool, port string) {
	h := server.Default(server.WithHostPorts("0.0.0.0:" + port))

	h.POST("/sse", func(ctx context.Context, c *app.RequestContext) {
		// client can tell server last event it received with Last-Event-ID header
		lastEventID := GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// you must set status code and response headers before first render call
		c.SetStatusCode(http.StatusOK)
		s := NewStream(c)
		body, err := c.Body()
		if err != nil {
			return
		}
		for a := 0; a < 10; a++ {
			s.Publish(&Event{Data: body})
		}
	})
	h.Run()
}

func publishMsgs(s *Stream, empty bool, count int) {
	for a := 0; a < count; a++ {
		if empty {
			s.Publish(&Event{Data: []byte("\n")})
		} else {
			s.Publish(&Event{Data: []byte("ping"), Retry: uint64(3000), Event: "test", ID: "1111"})
		}
	}
}

func publishMultilineMessages(s *Stream, count int) {
	for a := 0; a < count; a++ {
		s.Publish(&Event{ID: "123456", Data: []byte(mldata)})
	}
}

func wait(ch chan *Event, duration time.Duration) ([]byte, error) {
	var err error
	var msg []byte

	select {
	case event := <-ch:
		msg = event.Data
	case <-time.After(duration):
		err = errors.New("timeout")
	}
	return msg, err
}

func TestClientSubscribe(t *testing.T) {
	go newServer(false, "8886")
	time.Sleep(time.Second)
	tests := []struct {
		desc   string
		newCli func(t *testing.T) *Client
		newReq func() *protocol.Request
	}{
		{
			desc: "old interface",
			newCli: func(t *testing.T) *Client {
				return NewClient("http://127.0.0.1:8886/sse")
			},
		},
		{
			desc: "new interface",
			newCli: func(t *testing.T) *Client {
				hertzCli, err := client.NewClient()
				assert.Nil(t, err)
				cli, err := NewClientWithOptions(WithHertzClient(hertzCli))
				assert.Nil(t, err)
				return cli
			},
			newReq: func() *protocol.Request {
				req := &protocol.Request{}
				req.SetRequestURI("http://127.0.0.1:8886/sse")
				return req
			},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cli := test.newCli(t)

			// running multiple SSE requests concurrently
			var wg sync.WaitGroup
			for i := 0; i < 3; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					events := make(chan *Event)
					var cErr error
					go func() {
						var req *protocol.Request
						var sopts []SubscribeOption
						if test.newReq != nil {
							req = test.newReq()
							sopts = append(sopts, WithRequest(req))
						}
						cErr = cli.Subscribe(func(msg *Event) {
							if msg.Data != nil {
								events <- msg
								return
							}
						}, sopts...)
					}()

					for i := 0; i < 5; i++ {
						msg, err := wait(events, time.Second*1)
						assert.Nil(t, err)
						assert.DeepEqual(t, []byte(`ping`), msg)
					}

					assert.Nil(t, cErr)
				}()
			}
			wg.Wait()
		})
	}
}

func TestClientUnSubscribe(t *testing.T) {
	go newServer(false, "8887")
	time.Sleep(time.Second)
	tests := []struct {
		desc   string
		newCli func(t *testing.T) *Client
		newReq func() *protocol.Request
	}{
		{
			desc: "old interface",
			newCli: func(t *testing.T) *Client {
				return NewClient("http://127.0.0.1:8887/sse")
			},
		},
		{
			desc: "new interface",
			newCli: func(t *testing.T) *Client {
				hertzCli, err := client.NewClient()
				assert.Nil(t, err)
				cli, err := NewClientWithOptions(WithHertzClient(hertzCli))
				assert.Nil(t, err)
				return cli
			},
			newReq: func() *protocol.Request {
				req := &protocol.Request{}
				req.SetRequestURI("http://127.0.0.1:8887/sse")
				return req
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var req *protocol.Request
			var sopts []SubscribeOption
			cli := test.newCli(t)
			if test.newReq != nil {
				req = test.newReq()
				sopts = append(sopts, WithRequest(req))
			}

			events := make(chan *Event)
			ctx, cancel := context.WithCancel(context.Background())
			var cErr error
			go func() {
				cErr = cli.SubscribeWithContext(ctx, func(msg *Event) {
					if msg.Data != nil {
						events <- msg
						return
					}
				}, sopts...)
				assert.Nil(t, cErr)
			}()
			time.Sleep(1 * time.Second)
			cancel()
			time.Sleep(1 * time.Second)

			// read data that already published into channel
			_, _ = wait(events, time.Second*1)
			_, _ = wait(events, time.Second*1)

			// there is no event send to channel after calling cancel()
			for i := 0; i < 5; i++ {
				_, err := wait(events, time.Second*1)
				assert.DeepEqual(t, errors.New("timeout"), err)
			}
		})
	}
}

func TestClientSubscribeMultiline(t *testing.T) {
	go newMultilineServer("9007")
	time.Sleep(time.Second)
	tests := []struct {
		desc   string
		newCli func(t *testing.T) *Client
		newReq func() *protocol.Request
	}{
		{
			desc: "old interface",
			newCli: func(t *testing.T) *Client {
				return NewClient("http://127.0.0.1:9007/sse")
			},
		},
		{
			desc: "new interface",
			newCli: func(t *testing.T) *Client {
				hertzCli, err := client.NewClient()
				assert.Nil(t, err)
				cli, err := NewClientWithOptions(WithHertzClient(hertzCli))
				assert.Nil(t, err)
				return cli
			},
			newReq: func() *protocol.Request {
				req := &protocol.Request{}
				req.SetRequestURI("http://127.0.0.1:9007/sse")
				return req
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var req *protocol.Request
			var sopts []SubscribeOption
			cli := test.newCli(t)
			if test.newReq != nil {
				req = test.newReq()
				sopts = append(sopts, WithRequest(req))
			}

			events := make(chan *Event)
			var cErr error

			go func() {
				cErr = cli.Subscribe(func(msg *Event) {
					if msg.Data != nil {
						events <- msg
						return
					}
				}, sopts...)
			}()

			for i := 0; i < 5; i++ {
				msg, err := wait(events, time.Second*1)
				assert.Nil(t, err)
				assert.DeepEqual(t, []byte(mldata), msg)
			}

			assert.Nil(t, cErr)
		})
	}
}

func TestClientOnConnect(t *testing.T) {
	go newServerOnConnect(false, "9000")
	time.Sleep(time.Second)
	tests := []struct {
		desc   string
		newCli func(t *testing.T) *Client
		newReq func() *protocol.Request
	}{
		{
			desc: "old interface",
			newCli: func(t *testing.T) *Client {
				return NewClient("http://127.0.0.1:9000/sse")
			},
		},
		{
			desc: "new interface",
			newCli: func(t *testing.T) *Client {
				hertzCli, err := client.NewClient()
				assert.Nil(t, err)
				cli, err := NewClientWithOptions(WithHertzClient(hertzCli))
				assert.Nil(t, err)
				return cli
			},
			newReq: func() *protocol.Request {
				req := &protocol.Request{}
				req.SetRequestURI("http://127.0.0.1:9000/sse")
				return req
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var req *protocol.Request
			var sopts []SubscribeOption
			cli := test.newCli(t)
			if test.newReq != nil {
				req = test.newReq()
				sopts = append(sopts, WithRequest(req))
			}

			called := make(chan struct{})
			cli.SetOnConnectCallback(func(ctx context.Context, client *Client) {
				called <- struct{}{}
			})

			go cli.Subscribe(func(msg *Event) {}, sopts...)

			time.Sleep(time.Second)
			assert.DeepEqual(t, struct{}{}, <-called)
		})
	}
}

func TestClientUnsubscribe401(t *testing.T) {
	go newServer401("9009")
	time.Sleep(time.Second)
	tests := []struct {
		desc   string
		newCli func(t *testing.T) *Client
		newReq func() *protocol.Request
	}{
		{
			desc: "old interface",
			newCli: func(t *testing.T) *Client {
				return NewClient("http://127.0.0.1:9009/sse")
			},
		},
		{
			desc: "new interface",
			newCli: func(t *testing.T) *Client {
				hertzCli, err := client.NewClient()
				assert.Nil(t, err)
				cli, err := NewClientWithOptions(WithHertzClient(hertzCli))
				assert.Nil(t, err)
				return cli
			},
			newReq: func() *protocol.Request {
				req := &protocol.Request{}
				req.SetRequestURI("http://127.0.0.1:9009/sse")
				return req
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var req *protocol.Request
			var sopts []SubscribeOption
			cli := test.newCli(t)
			if test.newReq != nil {
				req = test.newReq()
				sopts = append(sopts, WithRequest(req))
			}

			err := cli.Subscribe(func(ev *Event) {
				// this shouldn't run
				assert.False(t, true)
			}, sopts...)

			assert.NotNil(t, err)
		})
	}
}

func TestClientLargeData(t *testing.T) {
	data := make([]byte, 1<<14)
	rand.Read(data)
	data = []byte(hex.EncodeToString(data))
	go newServerBigData(data, "9005")
	time.Sleep(time.Second)
	tests := []struct {
		desc   string
		newCli func(t *testing.T) *Client
		newReq func() *protocol.Request
	}{
		{
			desc: "old interface",
			newCli: func(t *testing.T) *Client {
				return NewClient("http://127.0.0.1:9005/sse")
			},
		},
		{
			desc: "new interface",
			newCli: func(t *testing.T) *Client {
				hertzCli, err := client.NewClient()
				assert.Nil(t, err)
				cli, err := NewClientWithOptions(WithHertzClient(hertzCli))
				assert.Nil(t, err)
				return cli
			},
			newReq: func() *protocol.Request {
				req := &protocol.Request{}
				req.SetRequestURI("http://127.0.0.1:9005/sse")
				return req
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var req *protocol.Request
			var sopts []SubscribeOption
			cli := test.newCli(t)
			if test.newReq != nil {
				req = test.newReq()
				sopts = append(sopts, WithRequest(req))
			}

			// limit retries to 3

			ec := make(chan *Event, 1)

			go func() {
				cli.Subscribe(func(ev *Event) {
					ec <- ev
				}, sopts...)
			}()

			d, err := wait(ec, time.Second)
			assert.Nil(t, err)
			assert.DeepEqual(t, data, d)
		})
	}
}

func TestTrimHeader(t *testing.T) {
	tests := []struct {
		input []byte
		want  []byte
	}{
		{
			input: []byte("data: real data"),
			want:  []byte("real data"),
		},
		{
			input: []byte("data:real data"),
			want:  []byte("real data"),
		},
		{
			input: []byte("data:"),
			want:  []byte(""),
		},
	}

	for _, tc := range tests {
		got := trimHeader(len(headerData), tc.input)
		assert.DeepEqual(t, tc.want, got)
	}
}

func TestRequestWithBody(t *testing.T) {
	go newServerWithPOSTBody(false, "9006")
	time.Sleep(time.Second)
	tests := []struct {
		desc   string
		newCli func(t *testing.T) *Client
		newReq func() *protocol.Request
	}{
		{
			desc: "old interface",
			newCli: func(t *testing.T) *Client {
				cli := NewClient("http://127.0.0.1:9006/sse")
				cli.SetMethod(consts.MethodPost)
				cli.SetBody([]byte(`{"msg":"echo"}`))
				return cli
			},
		},
		{
			desc: "new interface",
			newCli: func(t *testing.T) *Client {
				hertzCli, err := client.NewClient()
				assert.Nil(t, err)
				cli, err := NewClientWithOptions(WithHertzClient(hertzCli))
				assert.Nil(t, err)
				return cli
			},
			newReq: func() *protocol.Request {
				req := &protocol.Request{}
				req.SetRequestURI("http://127.0.0.1:9006/sse")
				req.SetMethod(consts.MethodPost)
				req.SetBody([]byte(`{"msg":"echo"}`))
				return req
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var req *protocol.Request
			var sopts []SubscribeOption
			cli := test.newCli(t)
			if test.newReq != nil {
				req = test.newReq()
				sopts = append(sopts, WithRequest(req))
			}

			events := make(chan *Event)
			var cErr error
			go func() {
				cErr = cli.Subscribe(func(msg *Event) {
					if msg.Data != nil {
						events <- msg
						return
					}
				}, sopts...)
			}()

			for i := 0; i < 5; i++ {
				msg, err := wait(events, time.Second*1)
				assert.Nil(t, err)
				assert.DeepEqual(t, []byte(`{"msg":"echo"}`), msg)
			}

			assert.Nil(t, cErr)
		})
	}
}

func TestNewClientWithOptions(t *testing.T) {
	// do not inject hertz client
	cli, err := NewClientWithOptions()
	assert.Nil(t, err)
	optsGetter, ok := cli.hertzClient.(interface{ GetOptions() *config.ClientOptions })
	assert.True(t, ok)
	opts := optsGetter.GetOptions()
	assert.NotNil(t, opts)
	assert.True(t, opts.ResponseBodyStream)

	// inject hertz client
	hCli, err := client.NewClient()
	assert.Nil(t, err)
	cli, err = NewClientWithOptions(WithHertzClient(hCli))
	assert.Nil(t, err)
	assert.DeepEqual(t, hCli, cli.hertzClient)
	optsGetter, ok = cli.hertzClient.(interface{ GetOptions() *config.ClientOptions })
	assert.True(t, ok)
	opts = optsGetter.GetOptions()
	assert.NotNil(t, opts)
	// NewClientWithOptions would configure ResponseBodyStream dynamically
	assert.True(t, opts.ResponseBodyStream)
}

type mockHertzClient struct {
	t            *testing.T
	expectMethod string
}

func (m *mockHertzClient) Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
	assert.DeepEqual(m.t, m.expectMethod, string(req.Method()))
	resp.Header = protocol.ResponseHeader{}
	resp.SetStatusCode(http.StatusOK)
	// inject empty reader so that the BodyStream could return io.EOF directly
	resp.SetBodyStream(strings.NewReader(""), 0)
	return nil
}

func TestWithRequest(t *testing.T) {
	cli, err := NewClientWithOptions(WithHertzClient(&mockHertzClient{t: t, expectMethod: consts.MethodGet}))
	assert.Nil(t, err)
	cli.SetMethod(consts.MethodPost)
	// req configured by WithRequest has higher priority
	req := &protocol.Request{}
	req.SetMethod(consts.MethodGet)
	err = cli.SubscribeWithContext(context.Background(), nil, WithRequest(req))
	assert.Nil(t, err)
}
