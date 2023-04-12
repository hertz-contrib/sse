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

package sse

import (
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

type streamBodyWriter struct {
	wroteHeader bool
	r           *protocol.Response
	w           network.Writer
}

func (b *streamBodyWriter) Write(p []byte) (n int, err error) {
	if !b.wroteHeader {
		if err = resp.WriteHeader(&b.r.Header, b.w); err != nil {
			return
		}
		b.wroteHeader = true
	}
	return b.w.WriteBinary(p)
}

func (b *streamBodyWriter) Flush() error {
	return b.w.Flush()
}

func (b *streamBodyWriter) Finalize() error {
	return nil
}

func NewStreamBodyWriter(r *protocol.Response, w network.Writer) network.ExtWriter {
	return &streamBodyWriter{
		r: r,
		w: w,
	}
}
