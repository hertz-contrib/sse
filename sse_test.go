/*
 * Copyright 2022 CloudWeGo Authors
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

// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package sse

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/network"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/stretchr/testify/assert"
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
			c.Response.SetConnectionClose()
			Stream(ctx, c, func(ctx context.Context, w network.ExtWriter) {
				for _, e := range expected {
					time.Sleep(time.Millisecond * 100)
					_ = Render(w, &e)
				}
				return
			})
		})
		h.Spin()
	}()

	for !h.IsRunning() {
		time.Sleep(time.Second)
	}
	c, err := client.NewClient(client.WithResponseBodyStream(true))
	assert.NoError(t, err)

	req := &protocol.Request{}
	rsp := &protocol.Response{}
	req.SetMethod(consts.MethodGet)
	req.SetRequestURI("http://127.0.0.1:8888/sse")
	err = c.Do(context.Background(), req, rsp)

	got, err := Decode(rsp.BodyStream())
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.Equal(t, ContentType, rsp.Header.Get("Content-Type"))
	assert.Equal(t, noCache, rsp.Header.Get("Cache-Control"))
	assert.Equal(t, rsp.StatusCode(), http.StatusOK)
	assert.Equal(t, got, expected)
}

func TestStreamRenderForever(t *testing.T) {
	h := server.Default()

	go func() {
		h.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
			Stream(ctx, c, func(ctx context.Context, w network.ExtWriter) {
				for t := range time.NewTicker(100 * time.Millisecond).C {
					event := &Event{Data: t.Format(time.RFC3339)}
					_ = Render(w, event)
				}
			})
		})
		h.Spin()
	}()

	for !h.IsRunning() {
		time.Sleep(time.Second)
	}
	c, err := client.NewClient(client.WithResponseBodyStream(true))
	assert.NoError(t, err)

	req := &protocol.Request{}
	rsp := &protocol.Response{}
	req.SetMethod(consts.MethodGet)

	req.SetRequestURI("http://127.0.0.1:8888/sse")
	err = c.DoTimeout(context.Background(), req, rsp, time.Second)
	// we expect timeout error because streaming never ends
	assert.NoError(t, err)
}

func TestLastEventID(t *testing.T) {
	var req app.RequestContext
	req.Request.Header.Set(LastEventID, "1")
	assert.Equal(t, "1", GetLastEventID(&req))
}

type NoOpsExtWriter struct{}

func (b NoOpsExtWriter) Write(_ []byte) (n int, err error) {
	return 0, nil
}

func (b NoOpsExtWriter) Flush() error {
	return nil
}

func (b NoOpsExtWriter) Finalize() error {
	return nil
}

func BenchmarkResponseWriter(b *testing.B) {
	var resp protocol.Response
	b.ResetTimer()
	b.ReportAllocs()
	resp.HijackWriter(&NoOpsExtWriter{})

	for i := 0; i < b.N; i++ {
		event := &Event{
			Event: "new_message",
			Data:  "hi! how are you? I am fine. this is a long stupid message!!!",
		}
		_ = Render(resp.GetHijackWriter(), event)
	}
}

func BenchmarkFullSSE(b *testing.B) {
	buf := new(bytes.Buffer)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = Encode(buf, &Event{
			Event: "new_message",
			ID:    "13435",
			Retry: 10,
			Data:  "hi! how are you? I am fine. this is a long stupid message!!!",
		})
		buf.Reset()
	}
}

func BenchmarkNoRetrySSE(b *testing.B) {
	buf := new(bytes.Buffer)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Encode(buf, &Event{
			Event: "new_message",
			ID:    "13435",
			Data:  "hi! how are you? I am fine. this is a long stupid message!!!",
		})
		buf.Reset()
	}
}

func BenchmarkSimpleSSE(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	buf := new(bytes.Buffer)

	for i := 0; i < b.N; i++ {
		_ = Encode(buf, &Event{
			Event: "new_message",
			Data:  "hi! how are you? I am fine. this is a long stupid message!!!",
		})
		buf.Reset()
	}
}
