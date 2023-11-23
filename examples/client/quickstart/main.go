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
package main

import (
	"context"
	"sync"

	"github.com/hertz-contrib/sse"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

var wg sync.WaitGroup

func main() {
	wg.Add(2)
	go func() {
		c := sse.NewClient("http://127.0.0.1:8888/sse")

		// touch off when connected to the server
		c.SetOnConnectCallback(func(ctx context.Context, client *sse.Client) {
			hlog.Infof("client1 connect to server %s success with %s method", c.GetURL(), c.GetMethod())
		})

		// touch off when the connection is shutdown
		c.SetDisconnectCallback(func(ctx context.Context, client *sse.Client) {
			hlog.Infof("client1 disconnect to server %s success with %s method", c.GetURL(), c.GetMethod())
		})

		events := make(chan *sse.Event)
		errChan := make(chan error)
		go func() {
			cErr := c.Subscribe(func(msg *sse.Event) {
				if msg.Data != nil {
					events <- msg
					return
				}
			})
			errChan <- cErr
		}()
		for {
			select {
			case e := <-events:
				hlog.Info(e)
			case err := <-errChan:
				hlog.CtxErrorf(context.Background(), "err = %s", err.Error())
				wg.Done()
				return
			}
		}
	}()

	go func() {
		c := sse.NewClient("http://127.0.0.1:8888/sse")

		// touch off when connected to the server
		c.SetOnConnectCallback(func(ctx context.Context, client *sse.Client) {
			hlog.Infof("client2 %s connect to server success with %s method", c.GetURL(), c.GetMethod())
		})

		// touch off when the connection is shutdown
		c.SetDisconnectCallback(func(ctx context.Context, client *sse.Client) {
			hlog.Infof("client2 %s disconnect to server success with %s method", c.GetURL(), c.GetMethod())
		})

		events := make(chan *sse.Event)
		errChan := make(chan error)
		go func() {
			cErr := c.Subscribe(func(msg *sse.Event) {
				if msg.Data != nil {
					events <- msg
					return
				}
			})
			errChan <- cErr
		}()
		for {
			select {
			case e := <-events:
				hlog.Info(e)
			case err := <-errChan:
				hlog.CtxErrorf(context.Background(), "err = %s", err.Error())
				wg.Done()
				return
			}
		}
	}()

	wg.Wait()
}
