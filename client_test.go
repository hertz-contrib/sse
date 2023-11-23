/*
 * Copyright 2023 CloudWeGo Authors
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
	"testing"
	"time"

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

func publishMsgs(s *Stream, empty bool, count int) {
	for a := 0; a < count; a++ {
		if empty {
			s.Publish(&Event{Data: []byte("\n")})
		} else {
			s.Publish(&Event{Data: []byte("ping")})
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
	c := NewClient("http://127.0.0.1:8886/sse")

	events := make(chan *Event)
	var cErr error
	go func() {
		cErr = c.Subscribe(func(msg *Event) {
			if msg.Data != nil {
				events <- msg
				return
			}
		})
	}()

	for i := 0; i < 5; i++ {
		msg, err := wait(events, time.Second*1)
		assert.Nil(t, err)
		assert.DeepEqual(t, []byte(`ping`), msg)
	}

	assert.Nil(t, cErr)
}

func TestClientSubscribeMultiline(t *testing.T) {
	go newMultilineServer("9007")
	time.Sleep(time.Second)
	c := NewClient("http://127.0.0.1:9007/sse")

	events := make(chan *Event)
	var cErr error

	go func() {
		cErr = c.Subscribe(func(msg *Event) {
			if msg.Data != nil {
				events <- msg
				return
			}
		})
	}()

	for i := 0; i < 5; i++ {
		msg, err := wait(events, time.Second*1)
		assert.Nil(t, err)
		assert.DeepEqual(t, []byte(mldata), msg)
	}

	assert.Nil(t, cErr)
}

func TestClientOnConnect(t *testing.T) {
	go newServerOnConnect(false, "9000")
	time.Sleep(time.Second)
	c := NewClient("http://127.0.0.1:9000/sse")

	called := make(chan struct{})
	c.SetOnConnectCallback(func(ctx context.Context, client *Client) {
		called <- struct{}{}
	})

	go c.Subscribe(func(msg *Event) {})

	time.Sleep(time.Second)
	assert.DeepEqual(t, struct{}{}, <-called)
}

func TestClientUnsubscribe401(t *testing.T) {
	go newServer401("9009")
	time.Sleep(time.Second)
	c := NewClient("http://127.0.0.1:9009/sse")

	err := c.Subscribe(func(ev *Event) {
		// this shouldn't run
		assert.False(t, true)
	})

	assert.NotNil(t, err)
}

func TestClientLargeData(t *testing.T) {
	data := make([]byte, 1<<14)
	rand.Read(data)
	data = []byte(hex.EncodeToString(data))
	go newServerBigData(data, "9005")
	time.Sleep(time.Second)
	c := NewClient("http://127.0.0.1:9005/sse")

	// limit retries to 3

	ec := make(chan *Event, 1)

	go func() {
		c.Subscribe(func(ev *Event) {
			ec <- ev
		})
	}()

	d, err := wait(ec, time.Second)
	assert.Nil(t, err)
	assert.DeepEqual(t, data, d)
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
