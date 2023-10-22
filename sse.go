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
	"strings"

	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/network"
)

// Server-Sent Events
// Last Updated 31 March 2023
// https://html.spec.whatwg.org/multipage/server-sent-events.html#server-sent-events

const (
	ContentType  = "text/event-stream"
	noCache      = "no-cache"
	cacheControl = "Cache-Control"
	LastEventID  = "Last-Event-ID"
)

var fieldReplacer = strings.NewReplacer(
	"\n", "\\n",
	"\r", "\\r")

var dataReplacer = strings.NewReplacer(
	"\n", "\ndata:",
	"\r", "\\r")

type Event struct {
	Event string
	ID    string
	Retry uint64
	Data  []byte
}

// GetLastEventID retrieve Last-Event-ID header if present.
func GetLastEventID(c *app.RequestContext) string {
	return c.Request.Header.Get(LastEventID)
}

type Stream struct {
	w network.ExtWriter
}

// NewStream creates a new stream for publishing Event.
func NewStream(c *app.RequestContext) *Stream {
	c.Response.Header.SetContentType(ContentType)
	if c.Response.Header.Get(cacheControl) == "" {
		c.Response.Header.Set(cacheControl, noCache)
	}

	writer := resp.NewChunkedBodyWriter(&c.Response, c.GetWriter())
	c.Response.HijackWriter(writer)
	return &Stream{
		writer,
	}
}

// Publish push an event to client.
func (c *Stream) Publish(event *Event) error {
	err := Encode(c.w, event)
	if err != nil {
		return err
	}
	return c.w.Flush()
}
