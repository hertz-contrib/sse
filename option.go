/*
 * Copyright 2025 CloudWeGo Authors
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
	"github.com/cloudwego/hertz/pkg/protocol"
	do "github.com/cloudwego/hertz/pkg/protocol/client"
)

type ClientOptions struct {
	hertzCli do.Doer
}

type ClientOption func(*ClientOptions)

// WithHertzClient specifies the underlying Hertz Client
func WithHertzClient(cli do.Doer) ClientOption {
	return func(opts *ClientOptions) {
		opts.hertzCli = cli
	}
}

type SubscribeOptions struct {
	req *protocol.Request
}

type SubscribeOption func(*SubscribeOptions)

// WithRequest specifies the request sent by the Hertz Client each time Subscribe or SubscribeWithContext is performed.
// Request-related configuration items set via client.SetXX will not take effect when WithRequest is specified:
// (client.SetURL, client.SetBody, client.SetMethod, client.SetHeaders).
func WithRequest(req *protocol.Request) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.req = req
	}
}
