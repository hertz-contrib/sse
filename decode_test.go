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
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestDecodeSingle1(t *testing.T) {
	events, err := Decode(bytes.NewBufferString(
		`data: this is a text
event: message
fake:
id: 123456789010
: we can append data
: and multiple comments should not break it
data: a very nice one`))

	assert.Nil(t, err)
	assert.Assert(t, len(events) == 1)
	assert.DeepEqual(t, events[0].Event, "message")
	assert.DeepEqual(t, events[0].ID, "123456789010")
}

func TestDecodeSingle2(t *testing.T) {
	events, err := Decode(bytes.NewBufferString(
		`: starting with a comment
fake:

data:this is a \ntext
event:a message\n\n
fake
:and multiple comments\n should not break it\n\n
id:1234567890\n10
:we can append data
data:a very nice one\n!


`))
	assert.Nil(t, err)
	assert.Assert(t, len(events) == 1)
	assert.DeepEqual(t, events[0].Event, "a message\\n\\n")
	assert.DeepEqual(t, events[0].ID, "1234567890\\n10")
}

func TestDecodeSingle3(t *testing.T) {
	events, err := Decode(bytes.NewBufferString(
		`
id:123456ABCabc789010
event: message123
: we can append data
data:this is a text
data: a very nice one
data:
data
: ending with a comment`))

	assert.Nil(t, err)
	assert.Assert(t, len(events) == 1)
	assert.DeepEqual(t, events[0].Event, "message123")
	assert.DeepEqual(t, events[0].ID, "123456ABCabc789010")
}

func TestDecodeMulti1(t *testing.T) {
	events, err := Decode(bytes.NewBufferString(
		`
id:
event: weird event
data:this is a text
:data: this should NOT APER
data:  second line

: a comment
event: message
id:123
data:this is a text
:data: this should NOT APER
data:  second line


: a comment
event: message
id:123
data:this is a text
data:  second line

:hola

data

event:

id`))
	assert.Nil(t, err)
	assert.DeepEqual(t, len(events), 3)
	assert.DeepEqual(t, events[0].Event, "weird event")
	assert.DeepEqual(t, events[0].ID, "")
}

func TestDecodeW3C(t *testing.T) {
	events, err := Decode(bytes.NewBufferString(
		`data

data
data

data:
`))
	assert.Nil(t, err)
	assert.DeepEqual(t, len(events), 1)
}
