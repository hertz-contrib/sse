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
 * The MIT License (MIT)
 *
 * Copyright (c) 2014 Manuel Martínez-Almeida
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
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
			Data:  []byte(strconv.FormatInt(int64(i), 10)),
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
			Data:  e.Data,
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

func BenchmarkNewStream(b *testing.B) {
	var c app.RequestContext
	var s *Stream
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s = NewStream(&c)
	}

	assert.DeepEqual(b, ContentType, string(c.Response.Header.ContentType()))
	assert.DeepEqual(b, noCache, c.Response.Header.Get(cacheControl))
	assert.NotNil(b, c.Response.GetHijackWriter())
	assert.NotNil(b, s)
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
		Data: []byte("hertz"),
	})
	assert.Nil(t, err)
	assert.DeepEqual(t, "data:hertz\n\n", buffer.String())
}

func BenchmarkStream_Publish(b *testing.B) {
	var c app.RequestContext
	s := NewStream(&c)
	var buffer bytes.Buffer
	s.w = &BufferExtWriter{
		buffer: &buffer,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Publish(&Event{
			Data: []byte("hertz"),
		})
	}
}

func TestLastEventID(t *testing.T) {
	var c app.RequestContext
	c.Request.Header.Set(LastEventID, "1")
	assert.DeepEqual(t, "1", GetLastEventID(&c))
}
