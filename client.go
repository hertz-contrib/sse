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
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/cloudwego/hertz/pkg/network/standard"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	headerID    = []byte("id:")
	headerData  = []byte("data:")
	headerEvent = []byte("event:")
	headerRetry = []byte("retry:")
)

// ConnCallback defines a function to be called on a particular connection event
type ConnCallback func(ctx context.Context, client *Client)

// ResponseValidator validates a response
type ResponseValidator func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error

// Client handles an incoming server stream
type Client struct {
	HertzClient        *client.Client
	disconnectCallback ConnCallback
	connectedCallback  ConnCallback
	ResponseValidator  ResponseValidator
	Headers            map[string]string
	URL                string
	Method             string
	maxBufferSize      int
	connected          bool
	EncodingBase64     bool
	LastEventID        atomic.Value // []byte
}

var defaultClient, _ = client.NewClient(client.WithDialer(standard.NewDialer()), client.WithResponseBodyStream(true))

// NewClient creates a new client
func NewClient(url string) *Client {
	c := &Client{
		URL:           url,
		HertzClient:   defaultClient,
		Headers:       make(map[string]string),
		maxBufferSize: 1 << 16,
		Method:        consts.MethodGet,
	}

	return c
}

// Subscribe to a data stream
func (c *Client) Subscribe(handler func(msg *Event)) error {
	return c.SubscribeWithContext(context.Background(), handler)
}

// SubscribeWithContext to a data stream with context
func (c *Client) SubscribeWithContext(ctx context.Context, handler func(msg *Event)) error {
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	err := c.request(ctx, req, resp)
	if err != nil {
		return err
	}
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(resp)
	}()
	if validator := c.ResponseValidator; validator != nil {
		err = validator(ctx, req, resp)
		if err != nil {
			return err
		}
	} else if resp.StatusCode() != consts.StatusOK {
		return fmt.Errorf("could not connect to stream code: %d", resp.StatusCode())
	}

	reader := NewEventStreamReader(resp.BodyStream(), c.maxBufferSize)
	eventChan, errorChan := c.startReadLoop(ctx, reader)

	for {
		select {
		case err = <-errorChan:
			return err
		case msg := <-eventChan:
			handler(msg)
		}
	}
}

func (c *Client) startReadLoop(ctx context.Context, reader *EventStreamReader) (chan *Event, chan error) {
	outCh := make(chan *Event)
	erChan := make(chan error)
	go c.readLoop(ctx, reader, outCh, erChan)
	return outCh, erChan
}

func (c *Client) readLoop(ctx context.Context, reader *EventStreamReader, outCh chan *Event, erChan chan error) {
	for {
		// Read each new line and process the type of event
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				erChan <- nil
				return
			}
			// run user specified disconnect function
			if c.disconnectCallback != nil {
				c.connected = false
				c.disconnectCallback(ctx, c)
			}
			erChan <- err
			return
		}

		if !c.connected && c.connectedCallback != nil {
			c.connected = true
			c.connectedCallback(ctx, c)
		}

		// If we get an error, ignore it.
		var msg *Event
		if msg, err = c.processEvent(event); err == nil {
			if len(msg.ID) > 0 {
				c.LastEventID.Store(msg.ID)
			} else {
				msg.ID, _ = c.LastEventID.Load().(string)
			}

			// Send downstream if the event has something useful
			if msg.hasContent() {
				outCh <- msg
			}
		}
	}
}

// OnDisconnect specifies the function to run when the connection disconnects
func (c *Client) OnDisconnect(fn ConnCallback) {
	c.disconnectCallback = fn
}

// OnConnect specifies the function to run when the connection is successful
func (c *Client) OnConnect(fn ConnCallback) {
	c.connectedCallback = fn
}

// SetMaxBufferSize  set sse client MaxBufferSize
func (c *Client) SetMaxBufferSize(size int) {
	c.maxBufferSize = size
}

func (c *Client) request(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
	req.SetMethod(c.Method)
	req.SetRequestURI(c.URL)

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	lastID, exists := c.LastEventID.Load().([]byte)
	if exists && lastID != nil {
		req.Header.Set(LastEventID, string(lastID))
	}
	// Add user specified headers
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	err := c.HertzClient.Do(ctx, req, resp)
	return err
}

func (c *Client) processEvent(msg []byte) (event *Event, err error) {
	var e Event

	if len(msg) < 1 {
		return nil, fmt.Errorf("event message was empty")
	}

	// Normalize the crlf to lf to make it easier to split the lines.
	// Split the line by "\n" or "\r", per the spec.
	for _, line := range bytes.FieldsFunc(msg, func(r rune) bool { return r == '\n' || r == '\r' }) {
		switch {
		case bytes.HasPrefix(line, headerID):
			e.ID = string(append([]byte(nil), trimHeader(len(headerID), line)...))
		case bytes.HasPrefix(line, headerData):
			// The spec allows for multiple data fields per event, concatenated them with "\n".
			e.Data = append(e.Data[:], append(trimHeader(len(headerData), line), byte('\n'))...)
		// The spec says that a line that simply contains the string "data" should be treated as a data field with an empty body.
		case bytes.Equal(line, bytes.TrimSuffix(headerData, []byte(":"))):
			e.Data = append(e.Data, byte('\n'))
		case bytes.HasPrefix(line, headerEvent):
			e.Event = string(append([]byte(nil), trimHeader(len(headerEvent), line)...))
		case bytes.HasPrefix(line, headerRetry):
			e.Retry = binary.BigEndian.Uint64(append([]byte(nil), trimHeader(len(headerRetry), line)...))
		default:
			// Ignore any garbage that doesn't match what we're looking for.
		}
	}

	// Trim the last "\n" per the spec.
	e.Data = bytes.TrimSuffix(e.Data, []byte("\n"))

	if c.EncodingBase64 {
		buf := make([]byte, base64.StdEncoding.DecodedLen(len(e.Data)))

		n, err := base64.StdEncoding.Decode(buf, e.Data)
		if err != nil {
			err = fmt.Errorf("failed to decode event message: %s", err)
			return &e, err
		}
		e.Data = buf[:n]
	}
	return &e, err
}

func trimHeader(size int, data []byte) []byte {
	if data == nil || len(data) < size {
		return data
	}

	data = data[size:]
	// Remove optional leading whitespace
	if len(data) > 0 && data[0] == 32 {
		data = data[1:]
	}
	// Remove trailing new line
	if len(data) > 0 && data[len(data)-1] == 10 {
		data = data[:len(data)-1]
	}
	return data
}
