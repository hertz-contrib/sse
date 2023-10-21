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
	"errors"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/cenkalti/backoff.v1"
)

var (
	headerID    = []byte("id:")
	headerData  = []byte("data:")
	headerEvent = []byte("event:")
	headerRetry = []byte("retry:")
)

func ClientMaxBufferSize(s int) func(c *Client) {
	return func(c *Client) {
		c.maxBufferSize = s
	}
}

// ConnCallback defines a function to be called on a particular connection event
type ConnCallback func(c *Client)

// ResponseValidator validates a response
type ResponseValidator func(c *Client, resp *protocol.Response) error

// Client handles an incoming server stream
type Client struct {
	Retry             time.Time
	ReconnectStrategy backoff.BackOff
	disconnectcb      ConnCallback
	connectedcb       ConnCallback
	subscribed        map[chan *Event]chan struct{}
	Headers           map[string]string
	ReconnectNotify   backoff.Notify
	ResponseValidator ResponseValidator
	HertzClient       *client.Client
	URL               string
	LastEventID       atomic.Value // []byte
	maxBufferSize     int
	mu                sync.Mutex
	EncodingBase64    bool
	Connected         bool
}

// NewClient creates a new client
func NewClient(url string, opts ...func(c *Client)) *Client {
	hertzClient, _ := client.NewClient(client.WithResponseBodyStream(true))
	c := &Client{
		URL:           url,
		HertzClient:   hertzClient,
		Headers:       make(map[string]string),
		subscribed:    make(map[chan *Event]chan struct{}),
		maxBufferSize: 1 << 16,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Subscribe to a data stream
func (c *Client) Subscribe(stream string, req *protocol.Request, handler func(msg *Event)) error {
	return c.SubscribeWithContext(context.Background(), req, stream, handler)
}

// SubscribeWithContext to a data stream with context
func (c *Client) SubscribeWithContext(ctx context.Context, req *protocol.Request, stream string, handler func(msg *Event)) error {
	operation := func() error {
		resp, err := c.request(ctx, req, stream)
		defer func() {
			protocol.ReleaseRequest(req)
			protocol.ReleaseResponse(resp)
		}()
		if err != nil {
			return err
		}
		if validator := c.ResponseValidator; validator != nil {
			err = validator(c, resp)
			if err != nil {
				return err
			}
		} else if resp.StatusCode() != 200 {
			resp.CloseBodyStream()
			return fmt.Errorf("could not connect to stream: %s", http.StatusText(resp.StatusCode()))
		}
		defer resp.CloseBodyStream()

		reader := NewEventStreamReader(resp.BodyStream(), c.maxBufferSize)
		eventChan, errorChan := c.startReadLoop(reader)

		for {
			select {
			case err = <-errorChan:
				return err
			case msg := <-eventChan:
				handler(msg)
			}
		}
	}

	// Apply user specified reconnection strategy or default to standard NewExponentialBackOff() reconnection method
	var err error
	if c.ReconnectStrategy != nil {
		err = backoff.RetryNotify(operation, c.ReconnectStrategy, c.ReconnectNotify)
	} else {
		err = backoff.RetryNotify(operation, backoff.NewExponentialBackOff(), c.ReconnectNotify)
	}
	return err
}

// SubscribeChan sends all events to the provided channel
func (c *Client) SubscribeChan(req *protocol.Request, stream string, ch chan *Event) error {
	return c.SubscribeChanWithContext(context.Background(), req, stream, ch)
}

// SubscribeChanWithContext sends all events to the provided channel with context
func (c *Client) SubscribeChanWithContext(ctx context.Context, req *protocol.Request, stream string, ch chan *Event) error {
	var connected bool
	errch := make(chan error)
	c.mu.Lock()
	c.subscribed[ch] = make(chan struct{})
	c.mu.Unlock()

	operation := func() error {
		resp, err := c.request(ctx, req, stream)
		defer func() {
			protocol.ReleaseRequest(req)
			protocol.ReleaseResponse(resp)
		}()
		if err != nil {
			return err
		}
		if validator := c.ResponseValidator; validator != nil {
			err = validator(c, resp)
			if err != nil {
				return err
			}
		} else if resp.StatusCode() != 200 {
			resp.CloseBodyStream()
			return fmt.Errorf("could not connect to stream: %s", http.StatusText(resp.StatusCode()))
		}
		defer resp.CloseBodyStream()

		if !connected {
			// Notify connect
			errch <- nil
			connected = true
		}

		reader := NewEventStreamReader(resp.BodyStream(), c.maxBufferSize)
		eventChan, errorChan := c.startReadLoop(reader)

		for {
			var msg *Event
			// Wait for message to arrive or exit
			select {
			case <-c.subscribed[ch]:
				return nil
			case err = <-errorChan:
				return err
			case msg = <-eventChan:
			}

			// Wait for message to be sent or exit
			if msg != nil {
				select {
				case <-c.subscribed[ch]:
					return nil
				case ch <- msg:
					// message sent
				}
			}
		}
	}

	go func() {
		defer c.cleanup(ch)
		// Apply user specified reconnection strategy or default to standard NewExponentialBackOff() reconnection method
		var err error
		if c.ReconnectStrategy != nil {
			err = backoff.RetryNotify(operation, c.ReconnectStrategy, c.ReconnectNotify)
		} else {
			err = backoff.RetryNotify(operation, backoff.NewExponentialBackOff(), c.ReconnectNotify)
		}

		// channel closed once connected
		if err != nil && !connected {
			errch <- err
		}
	}()
	err := <-errch
	close(errch)
	return err
}

func (c *Client) startReadLoop(reader *EventStreamReader) (chan *Event, chan error) {
	outCh := make(chan *Event)
	erChan := make(chan error)
	go c.readLoop(reader, outCh, erChan)
	return outCh, erChan
}

func (c *Client) readLoop(reader *EventStreamReader, outCh chan *Event, erChan chan error) {
	for {
		// Read each new line and process the type of event
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				erChan <- nil
				return
			}
			// run user specified disconnect function
			if c.disconnectcb != nil {
				c.Connected = false
				c.disconnectcb(c)
			}
			erChan <- err
			return
		}

		if !c.Connected && c.connectedcb != nil {
			c.Connected = true
			c.connectedcb(c)
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

// SubscribeRaw to an sse endpoint
func (c *Client) SubscribeRaw(req *protocol.Request, handler func(msg *Event)) error {
	return c.Subscribe("", req, handler)
}

// SubscribeRawWithContext to an sse endpoint with context
func (c *Client) SubscribeRawWithContext(ctx context.Context, req *protocol.Request, handler func(msg *Event)) error {
	return c.SubscribeWithContext(ctx, req, "", handler)
}

// SubscribeChanRaw sends all events to the provided channel
func (c *Client) SubscribeChanRaw(req *protocol.Request, ch chan *Event) error {
	return c.SubscribeChan(req, "", ch)
}

// SubscribeChanRawWithContext sends all events to the provided channel with context
func (c *Client) SubscribeChanRawWithContext(ctx context.Context, req *protocol.Request, ch chan *Event) error {
	return c.SubscribeChanWithContext(ctx, req, "", ch)
}

// Unsubscribe unsubscribes a channel
func (c *Client) Unsubscribe(ch chan *Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.subscribed[ch] != nil {
		c.subscribed[ch] <- struct{}{}
	}
}

// OnDisconnect specifies the function to run when the connection disconnects
func (c *Client) OnDisconnect(fn ConnCallback) {
	c.disconnectcb = fn
}

// OnConnect specifies the function to run when the connection is successful
func (c *Client) OnConnect(fn ConnCallback) {
	c.connectedcb = fn
}

func (c *Client) request(ctx context.Context, req *protocol.Request, stream string) (*protocol.Response, error) {
	resp := protocol.AcquireResponse()
	req.SetMethod("GET")
	req.SetRequestURI(c.URL)

	//req, err := adaptor.GetCompatRequest(hReq)
	//if err != nil {
	//	return nil, err
	//}

	//req, err := http.NewRequest("GET", c.URL, nil)
	//if err != nil {
	//	return nil, err
	//}
	//req = req.WithContext(ctx)

	// Setup request, specify stream to connect to
	if stream != "" {
		req.URI().QueryArgs().Add("stream", stream)
		// 不确定是否已经加入到原始请求 url 中，有待测试
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	lastID, exists := c.LastEventID.Load().([]byte)
	if exists && lastID != nil {
		req.Header.Set("Last-Event-ID", string(lastID))
	}

	// Add user specified headers
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	err := c.HertzClient.Do(ctx, req, resp)
	return resp, err

}

func (c *Client) processEvent(msg []byte) (event *Event, err error) {
	var e Event

	if len(msg) < 1 {
		return nil, errors.New("event message was empty")
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
			// 这里采用的是大端序
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
		}
		e.Data = buf[:n]
	}
	return &e, err
}

func (c *Client) cleanup(ch chan *Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.subscribed[ch] != nil {
		close(c.subscribed[ch])
		delete(c.subscribed, ch)
	}
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
