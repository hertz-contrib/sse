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
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gopkg.in/cenkalti/backoff.v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var urlPath = "http://127.0.0.1:8888/sse"

var mldata = `{
	"key": "value",
	"array": [
		1,
		2,
		3
	]
}`

func setup(empty bool) {
	go newServer(empty)
}

func setupWithContext(ctx context.Context, empty bool) {
	go newServerWithContext(ctx, empty)
}

func setupMultiline() {
	go newMultilineServer()
}

func setupCount(empty bool, count int) {
	go newServerCount(empty, count)
}

func setupDisconnect(empty bool) {
	go newServerDisconnect(empty)
}

func newServer(empty bool) {
	h := server.Default()

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

func newServerWithContext(ctx context.Context, empty bool) {
	h := server.Default()

	h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		// client can tell server last event it received with Last-Event-ID header
		lastEventID := GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// you must set status code and response headers before first render call
		c.SetStatusCode(http.StatusOK)
		s := NewStream(c)
		publishMsgs(s, empty, 10000)
	})
	go func() {
		select {
		case <-ctx.Done():
			err := h.Close()
			if err != nil {
				fmt.Println(h.IsRunning())
				fmt.Println(err)
			}
		}
	}()
	if err := h.Run(); err != nil {
		// ...
		panic(err)
	}
}

func newServerBigData(data []byte) {
	h := server.Default()

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

func newServerCount(empty bool, count int) {
	h := server.Default()

	h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		// client can tell server last event it received with Last-Event-ID header
		lastEventID := GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// you must set status code and response headers before first render call
		c.SetStatusCode(http.StatusOK)
		s := NewStream(c)
		publishMsgs(s, empty, count)
	})

	h.Spin()
}

func newMultilineServer() {
	h := server.Default()

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

func newServerDisconnect(empty bool) {
	h := server.Default()

	h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		// client can tell server last event it received with Last-Event-ID header
		lastEventID := GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// you must set status code and response headers before first render call
		c.SetStatusCode(http.StatusOK)
		s := NewStream(c)
		go func() {
			time.Sleep(time.Second)
			c.SetConnectionClose()
			h.Close()
			fmt.Println(h.IsRunning())
		}()
		publishMsgs(s, empty, 1000)
	})
	h.Run()
}

func newServer401() {
	h := server.Default()

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

func waitEvent(ch chan *Event, duration time.Duration) (*Event, error) {
	select {
	case event := <-ch:
		return event, nil
	case <-time.After(duration):
		return nil, errors.New("timeout")
	}
}

func TestClientSubscribe(t *testing.T) {
	setup(false)

	c := NewClient(urlPath)

	events := make(chan *Event)
	var cErr error
	go func() {
		cErr = c.Subscribe("test", func(msg *Event) {
			if msg.Data != nil {
				events <- msg
				return
			}
		})
	}()

	for i := 0; i < 5; i++ {
		msg, err := wait(events, time.Second*1)
		require.Nil(t, err)
		assert.Equal(t, []byte(`ping`), msg)
	}

	assert.Nil(t, cErr)
}

func TestClientSubscribeMultiline(t *testing.T) {
	setupMultiline()

	c := NewClient(urlPath)

	events := make(chan *Event)
	var cErr error

	go func() {
		cErr = c.Subscribe("test", func(msg *Event) {
			if msg.Data != nil {
				events <- msg
				return
			}
		})
	}()

	for i := 0; i < 5; i++ {
		msg, err := wait(events, time.Second*1)
		require.Nil(t, err)
		assert.Equal(t, []byte(mldata), msg)
	}

	assert.Nil(t, cErr)
}

func TestClientChanSubscribeEmptyMessage(t *testing.T) {
	setup(true)

	c := NewClient(urlPath)

	events := make(chan *Event)
	err := c.SubscribeChan("test", events)
	require.Nil(t, err)

	for i := 0; i < 5; i++ {
		_, err := waitEvent(events, time.Second)
		require.Nil(t, err)
	}
}

func TestClientChanSubscribe(t *testing.T) {
	setup(false)

	c := NewClient(urlPath)

	events := make(chan *Event)
	err := c.SubscribeChan("test", events)
	require.Nil(t, err)

	for i := 0; i < 5; i++ {
		msg, merr := wait(events, time.Second*1)
		if msg == nil {
			i--
			continue
		}
		assert.Nil(t, merr)
		assert.Equal(t, []byte(`ping`), msg)
	}
	c.Unsubscribe(events)
}

//func TestClientOnDisconnect(t *testing.T) {
//	setupDisconnect(false)
//
//	c := NewClient(urlPath)
//
//	called := make(chan struct{})
//	c.OnDisconnect(func(client *SSEClient) {
//		called <- struct{}{}
//	})
//	go c.Subscribe("test", func(msg *Event) {})
//
//	time.Sleep(time.Second)
//	//c.HertzClient.CloseIdleConnections()
//	//server.CloseClientConnections()
//
//	assert.Equal(t, struct{}{}, <-called)
//}

func TestClientOnConnect(t *testing.T) {
	setup(false)

	c := NewClient(urlPath)

	called := make(chan struct{})
	c.OnConnect(func(client *SSEClient) {
		called <- struct{}{}
	})

	go c.Subscribe("test", func(msg *Event) {})

	time.Sleep(time.Second)
	assert.Equal(t, struct{}{}, <-called)
}

func TestClientChanReconnect(t *testing.T) {
	setup(false)

	c := NewClient(urlPath)

	events := make(chan *Event)
	err := c.SubscribeChan("test", events)
	require.Nil(t, err)

	for i := 0; i < 10; i++ {
		if i == 5 {
			// kill connection
		}
		msg, merr := wait(events, time.Second*1)
		if msg == nil {
			i--
			continue
		}
		assert.Nil(t, merr)
		assert.Equal(t, []byte(`ping`), msg)
	}
	c.Unsubscribe(events)
}

func TestClientUnsubscribe(t *testing.T) {
	setup(false)

	c := NewClient(urlPath)

	events := make(chan *Event)
	err := c.SubscribeChan("test", events)
	require.Nil(t, err)

	time.Sleep(time.Millisecond * 500)

	go c.Unsubscribe(events)
	go c.Unsubscribe(events)
}

func TestClientUnsubscribeNonBlock(t *testing.T) {
	count := 2
	setupCount(false, count)

	c := NewClient(urlPath)

	events := make(chan *Event)
	err := c.SubscribeChan("test", events)
	require.Nil(t, err)

	// Read count messages from the channel
	for i := 0; i < count; i++ {
		msg, merr := wait(events, time.Second*1)
		assert.Nil(t, merr)
		assert.Equal(t, []byte(`ping`), msg)
	}
	// No more data is available to be read in the channel
	// Make sure Unsubscribe returns quickly
	doneCh := make(chan *Event)
	go func() {
		var e Event
		c.Unsubscribe(events)
		doneCh <- &e
	}()
	_, merr := wait(doneCh, time.Millisecond*100)
	assert.Nil(t, merr)
}

func TestClientUnsubscribe401(t *testing.T) {
	go newServer401()

	c := NewClient(urlPath)

	// limit retries to 3
	c.ReconnectStrategy = backoff.WithMaxTries(
		backoff.NewExponentialBackOff(),
		3,
	)

	err := c.SubscribeRaw(func(ev *Event) {
		// this shouldn't run
		assert.False(t, true)
	})

	require.NotNil(t, err)
}

func TestClientLargeData(t *testing.T) {
	data := make([]byte, 1<<14)
	rand.Read(data)
	data = []byte(hex.EncodeToString(data))
	go newServerBigData(data)

	c := NewClient(urlPath)

	// limit retries to 3
	c.ReconnectStrategy = backoff.WithMaxTries(
		backoff.NewExponentialBackOff(),
		3,
	)

	ec := make(chan *Event, 1)

	go func() {
		c.Subscribe("test", func(ev *Event) {
			ec <- ev
		})
	}()

	d, err := wait(ec, time.Second)
	require.Nil(t, err)
	require.Equal(t, data, d)
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
		require.Equal(t, tc.want, got)
	}
}

func TestSubscribeWithContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	setupWithContext(ctx, false)
	time.Sleep(2 * time.Second)
	n1 := runtime.NumGoroutine()
	c := NewClient(urlPath)
	for i := 0; i < 10; i++ {
		go c.SubscribeWithContext(ctx, "test", func(msg *Event) {})
	}
	time.Sleep(1 * time.Second)
	cancel()
	c.HertzClient.CloseIdleConnections()
	time.Sleep(1 * time.Second)
	n2 := runtime.NumGoroutine()
	assert.Equal(t, n1+1, n2) // protocol.refreshServerDate() creates an goroutine to refreshServerDate that can not be canceled
}
