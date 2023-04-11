package hertz_contrib_sse

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

type myStruct struct {
	A int
	B string `json:"value"`
}

func TestEncode(t *testing.T) {
	test := []struct {
		Name    string
		Event   *Event
		WantErr byte
		Want    string
	}{
		{
			Name: "with data only",
			Want: `data:junk
data:
data:jk
data:id:fake

`,
			Event: &Event{
				Data: "junk\n\njk\nid:fake",
			},
		},
		{
			Name: "with event",
			Want: `event:t\n:<>\r	est
data:junk
data:
data:jk
data:id:fake

`,
			Event: &Event{
				Event: "t\n:<>\r\test",
				Data:  "junk\n\njk\nid:fake",
			},
		},
		{
			Name: "with id",
			Event: &Event{
				ID:   "t\n:<>\r\test",
				Data: "junk\n\njk\nid:fa\rke",
			},
			Want: `id:t\n:<>\r	est
data:junk
data:
data:jk
data:id:fa\rke

`,
		},
		{
			Name: "with retry",
			Event: &Event{
				Retry: 11,
				Data:  "junk\n\njk\nid:fake\n",
			},
			Want: `retry:11
data:junk
data:
data:jk
data:id:fake
data:

`,
		},
		{
			Name: "with everything",
			Event: &Event{
				Event: "abc",
				ID:    "12345",
				Retry: 10,
				Data:  "some data",
			},
			Want: "id:12345\nevent:abc\nretry:10\ndata:some data\n\n",
		},
		{
			Name: "encode map",
			Event: &Event{
				Event: "a slice",
				Data:  myStruct{1, "number"},
			},
			Want: "event:a slice\ndata:{\"A\":1,\"value\":\"number\"}\n\n",
		},
		{
			Name: "encode struct",
			Event: &Event{
				Event: "a struct",
				Data:  myStruct{1, "number"},
			},
			Want: "event:a struct\ndata:{\"A\":1,\"value\":\"number\"}\n\n",
		},
		{
			Name: "struct pointer",
			Event: &Event{
				Event: "a struct",
				Data:  &myStruct{1, "number"},
			},
			Want: "event:a struct\ndata:{\"A\":1,\"value\":\"number\"}\n\n",
		},
		{
			Name: "encode integer",
			Event: &Event{
				Event: "an integer",
				Data:  1,
			},
			Want: "event:an integer\ndata:1\n\n",
		},
		{
			Name: "encode float",
			Event: &Event{
				Event: "Float",
				Data:  1.5,
			},
			Want: "event:Float\ndata:1.5\n\n",
		},
		{
			Name: "encode string",
			Event: &Event{
				Event: "String",
				Data:  "hertz",
			},
			Want: "event:String\ndata:hertz\n\n",
		},
	}
	for _, tt := range test {
		var b bytes.Buffer
		err := Encode(&b, tt.Event)
		got := b.String()
		assert.NoError(t, err)
		assert.Equal(t, string(got), tt.Want)
	}
}

func TestEncodeStream(t *testing.T) {
	w := new(bytes.Buffer)
	event := &Event{
		Event: "float",
		Data:  "1.5",
	}
	err := Encode(w, event)
	assert.NoError(t, err)

	event = &Event{
		ID:   "123",
		Data: map[string]interface{}{"foo": "bar", "bar": "foo"},
	}
	err = Encode(w, event)
	assert.NoError(t, err)

	event = &Event{
		ID:    "124",
		Event: "chat",
		Data:  "hi! dude",
	}
	err = Encode(w, event)
	assert.NoError(t, err)
	assert.Equal(t, "event:float\ndata:1.5\n\nid:123\ndata:{\"bar\":\"foo\",\"foo\":\"bar\"}\n\nid:124\nevent:chat\ndata:hi! dude\n\n", w.String())
}
