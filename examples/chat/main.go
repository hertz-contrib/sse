package main

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/network"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/hertz-contrib/sse"
)

// ChatServer implements a chat server to demonstrate use of server sent event in hertz.
// It supports direct messaging and broadcast messaging.
type ChatServer struct {
	// BroadcastMessageC Broadcast messages are pushed to this channel
	BroadcastMessageC chan ChatMessage
	// DirectMessageC direct messages are pushed to this channel
	DirectMessageC chan ChatMessage
	// Receive keeps a list of chan ChatMessage, one per user
	Receive map[string]chan ChatMessage
}

type ChatMessage struct {
	Type      string
	From      string
	To        string
	Message   string
	Timestamp time.Time
}

func main() {
	h := server.Default()

	// initialize new chat server
	srv := NewServer()

	// create receive channel for each user
	h.Use(srv.CreateReceiveChannel())

	// add event-streaming headers
	h.GET("/chat/sse", srv.ServerSentEvent)

	h.POST("/chat/broadcast", srv.Broadcast)

	h.POST("/chat/direct", srv.Direct)

	h.Spin()
}

func NewServer() (srv *ChatServer) {
	srv = &ChatServer{
		BroadcastMessageC: make(chan ChatMessage),
		DirectMessageC:    make(chan ChatMessage),
		Receive:           make(map[string]chan ChatMessage),
	}

	go srv.relay()

	return
}

func (srv *ChatServer) ServerSentEvent(ctx context.Context, c *app.RequestContext) {
	// in production, you would get user's identity in other ways e.g. Authorization
	username := c.Query("username")

	// get messages from user's receive channel
	sse.Stream(ctx, c, func(ctx context.Context, w network.ExtWriter) {
		for msg := range srv.Receive[username] {

			payload, err := json.Marshal(msg)
			if err != nil {
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}
			hlog.CtxInfof(ctx, "message received: %+v", msg)
			event := &sse.Event{
				Event: msg.Type,
				Data:  string(payload),
			}
			c.SetStatusCode(http.StatusOK)
			err = event.Render(w)
			if err != nil {
				return
			}
		}
	})
}

func (srv *ChatServer) Direct(ctx context.Context, c *app.RequestContext) {
	// in production, you would get user's identity in other ways e.g. Authorization
	from := c.Query("from")
	to := c.Query("to")
	message := c.Query("message")

	msg := ChatMessage{
		From:      from,
		To:        to,
		Message:   message,
		Type:      "direct",
		Timestamp: time.Now(),
	}
	// deliver message to DirectMessageC.
	srv.DirectMessageC <- msg

	hlog.CtxInfof(ctx, "message sent: %+v", msg)
	c.JSON(200, utils.H{
		"message": "success",
	})
}

func (srv *ChatServer) Broadcast(ctx context.Context, c *app.RequestContext) {
	// in production, you would get user's identity in other ways e.g. Authorization
	from := c.Query("from")
	message := c.Query("message")

	msg := ChatMessage{
		From:      from,
		Message:   message,
		Type:      "broadcast",
		Timestamp: time.Now(),
	}
	// deliver message to BroadcastMessageC.
	srv.BroadcastMessageC <- msg

	hlog.CtxInfof(ctx, "message sent: %+v", msg)
	c.JSON(200, utils.H{
		"message": "success",
	})
}

// relay handles messages sent to BroadcastMessageC and DirectMessageC and
// relay messages to receive channels depends on message type.
func (srv *ChatServer) relay() {
	for {
		select {
		// broadcast message to all users
		case msg := <-srv.BroadcastMessageC:
			for _, r := range srv.Receive {
				r <- msg
			}

		// deliver message to user specified in To
		case msg := <-srv.DirectMessageC:
			srv.Receive[msg.To] <- msg
		}
	}
}

// CreateReceiveChannel creates a buffered receive channel for each user.
func (srv *ChatServer) CreateReceiveChannel() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		username := c.Query("username")
		// if user doesn't have a channel yet, create a new one.
		if receive, found := srv.Receive[username]; !found {
			receive = make(chan ChatMessage, 1000)
			srv.Receive[username] = receive
		}
		c.Next(ctx)
	}
}
