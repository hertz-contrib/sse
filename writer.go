package hertz_contrib_sse

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
