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

package hertz_contrib_sse

import (
	"context"
	"fmt"
	"strings"

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
	Retry uint
	Data  any
}

type HandlerFunc func(ctx context.Context, w network.ExtWriter)

// Stream set headers for server-sent events and ensure response writer is set,
// hijack with StreamBodyWriter otherwise
func Stream(ctx context.Context, c *app.RequestContext, f HandlerFunc) {
	c.Response.Header.SetContentType(ContentType)
	if c.Response.Header.Get(cacheControl) == "" {
		c.Response.Header.Set(cacheControl, noCache)
	}
	c.Response.ImmediateHeaderFlush = true
	writer := c.Response.GetHijackWriter()
	// set stream body writer
	if writer == nil {
		writer = NewStreamBodyWriter(&c.Response, c.GetWriter())
		c.Response.HijackWriter(writer)
	}
	f(ctx, writer)
}

// Render encodes Event to bytes and send it to peer with immediate Flush.
func (e *Event) Render(w network.ExtWriter) error {
	err := Encode(w, e)
	if err != nil {
		return fmt.Errorf("encode error: %w", err)
	}
	return w.Flush()
}

// GetLastEventID retrieve Last-Event-ID header if present
func GetLastEventID(c *app.RequestContext) string {
	return c.Request.Header.Get(LastEventID)
}
