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
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/r3labs/sse/v2"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestStreamRender(t *testing.T) {
	h := server.Default()
	var expected []Event
	for i := 0; i < 10; i++ {
		expected = append(expected, Event{
			Event: "counter",
			Data:  strconv.FormatInt(int64(i), 10),
		})
	}

	go func() {
		h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
			stream := NewStream(c)
			for _, e := range expected {
				time.Sleep(time.Millisecond * 100)
				_ = stream.Publish(&e)
			}
		})
		h.Spin()
	}()

	for !h.IsRunning() {
		time.Sleep(time.Second)
	}

	events := make(chan *sse.Event)

	c := sse.NewClient("http://127.0.0.1:8888/sse")
	err := c.SubscribeChan("counter", events)
	assert.Nil(t, err)

	var got []Event
	for e := range events {
		got = append(got, Event{
			Event: string(e.Event),
			Data:  string(e.Data),
		})
		if len(got) == len(expected) {
			close(events)
		}
	}

	assert.DeepEqual(t, expected, got)
}

func TestNewStream(t *testing.T) {
	var c app.RequestContext
	s := NewStream(&c)

	assert.DeepEqual(t, ContentType, string(c.Response.Header.ContentType()))
	assert.DeepEqual(t, noCache, c.Response.Header.Get(cacheControl))
	assert.NotNil(t, c.Response.GetHijackWriter())
	assert.NotNil(t, s)
}

type BufferExtWriter struct {
	buffer *bytes.Buffer
}

func (b BufferExtWriter) Write(p []byte) (n int, err error) {
	return b.buffer.Write(p)
}

func (b BufferExtWriter) Flush() error {
	return nil
}

func (b BufferExtWriter) Finalize() error {
	return nil
}

func TestStreamPublish(t *testing.T) {
	var c app.RequestContext
	s := NewStream(&c)
	var buffer bytes.Buffer
	s.w = &BufferExtWriter{
		buffer: &buffer,
	}
	err := s.Publish(&Event{
		Data: "hertz",
	})
	assert.Nil(t, err)
	assert.DeepEqual(t, "data:hertz\n\n", buffer.String())
}

func TestLastEventID(t *testing.T) {
	var c app.RequestContext
	c.Request.Header.Set(LastEventID, "1")
	assert.DeepEqual(t, "1", GetLastEventID(&c))
}
