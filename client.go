package sse

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	hertz "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"log"
	"net/http"
	"strings"
	"time"
)

//	type Client struct {
//		Retry             time.Time
//		ReconnectStrategy backoff.BackOff
//		disconnectcb      ConnCallback
//		connectedcb       ConnCallback
//		subscribed        map[chan *Event]chan struct{}
//		Headers           map[string]string
//		ReconnectNotify   backoff.Notify
//		ResponseValidator ResponseValidator
//		Connection        *http.Client
//		URL               string
//		LastEventID       atomic.Value // []byte
//		maxBufferSize     int
//		mu                sync.Mutex
//		EncodingBase64    bool
//		Connected         bool
//	}
type Client struct {
	hertzClient *hertz.Client
}

type EventStreamReader struct {
	scanner *bufio.Scanner
}

// NewClient creates a new client
func NewClient() *Client {
	client := hertz.Client{}

	return &Client{hertzClient: &client}
}

func (c *Client) Subscribe(ctx context.Context, req *protocol.Request) error {
	// 设置 SSE 请求头
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	// 发起 SSE 连接
	var resp *protocol.Response
	err := c.hertzClient.Do(ctx, req, resp)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.CloseBodyStream()

	if resp.StatusCode() != 200 {
		resp.CloseBodyStream()
		return fmt.Errorf("could not connect to stream: %s", http.StatusText(resp.StatusCode()))
	}

	// 读取 SSE 响应
	go func() {
		decoder := NewEventStreamReader(resp.Body)

		// 逐行读取 SSE 事件
		for {
			event, err := decoder.DecodeEvent()
			if err != nil {
				log.Fatal(err)
			}

			// 处理 SSE 事件
			fmt.Println("Received event:")
			fmt.Println("Event:", event.Event)
			fmt.Println("Data:", event.Data)
			fmt.Println()
		}
	}()

	// 等待一段时间，模拟 SSE 连接的持续性
	time.Sleep(5 * time.Minute)

	return nil
}

func NewEventStreamReader(body func() []byte) *EventStreamReader {

}

func (d *EventStreamReader) DecodeEvent() (*Event, error) {
	var event Event

	for d.scanner.Scan() {
		line := d.scanner.Text()

		// 空行表示一个事件的结束
		if line == "" {
			return &event, nil
		}

		// 根据 SSE 事件属性前缀进行解析
		if strings.HasPrefix(line, "event:") {
			event.Event = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "data:") {
			//
		} else if strings.HasPrefix(line, "id:") {
			//
		} else if strings.HasPrefix(line, "retry:") {
			//
		}
	}

	if err := d.scanner.Err(); err != nil {
		return nil, err
	}

	return nil, errors.New("unexpected end of SSE stream")
}
