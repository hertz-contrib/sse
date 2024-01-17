// Copyright 2023 CloudWeGo Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/hertz-contrib/sse"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// It keeps a list of clients those are currently attached
// and broadcasting events to those clients.
type Server struct {
	// Events are pushed to this channel by the main events-gathering routine
	Price chan sse.Event

	// New client connections
	NewClients chan chan sse.Event

	// Closed client connections
	ClosedClients chan chan sse.Event

	// Total client connections
	TotalClients map[chan sse.Event]bool
}

// New event messages are broadcast to all registered client connection channels
type ClientChan chan sse.Event

func main() {
	h := server.Default()

	// Initialize new streaming server
	srv := NewServer()

	// We are streaming current time to clients in the interval 10 seconds
	go func() {
		for {
			time.Sleep(1 * time.Second)
			now := time.Now()
			for _, stock := range []string{"AAPL", "AMZN"} {
				// Send current time to clients message channel
				srv.Price <- sse.Event{Event: stock, ID: strconv.FormatInt(now.UnixMilli(), 10), Data: []byte(fmt.Sprintf("%f", rand.Float64()*100))}
			}
		}
	}()

	// Authorized client can stream the event
	// Add event-streaming headers
	h.GET("/price", srv.serveHTTP(), func(ctx context.Context, c *app.RequestContext) {
		v, ok := c.Get("clientChan")
		if !ok {
			return
		}
		clientChan, ok := v.(ClientChan)
		if !ok {
			return
		}
		c.SetStatusCode(http.StatusOK)
		stream := sse.NewStream(c)
		for event := range clientChan {
			err := stream.Publish(&event)
			if err != nil {
				return
			}
		}
	})

	h.Spin()
}

// Initialize event and Start procnteessing requests
func NewServer() (srv *Server) {
	srv = &Server{
		Price:         make(chan sse.Event),
		NewClients:    make(chan chan sse.Event),
		ClosedClients: make(chan chan sse.Event),
		TotalClients:  make(map[chan sse.Event]bool),
	}

	go srv.listen()

	return
}

// It Listens all incoming requests from clients.
// Handles addition and removal of clients and broadcast messages to clients.
func (srv *Server) listen() {
	for {
		select {
		// Add new available client
		case client := <-srv.NewClients:
			srv.TotalClients[client] = true
			log.Printf("Client added. %d registered clients", len(srv.TotalClients))

		// Remove closed client
		case client := <-srv.ClosedClients:
			delete(srv.TotalClients, client)
			close(client)
			log.Printf("Removed client. %d registered clients", len(srv.TotalClients))

		// Broadcast message to client
		case event := <-srv.Price:
			for clientMessageChan := range srv.TotalClients {
				clientMessageChan <- event
			}
		}
	}
}

func (srv *Server) serveHTTP() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// Initialize client channel
		clientChan := make(ClientChan, 1)

		// Send new connection to event server
		srv.NewClients <- clientChan

		defer func() {
			// Send closed connection to event server
			srv.ClosedClients <- clientChan
		}()

		c.Set("clientChan", clientChan)

		c.Next(ctx)
	}
}
