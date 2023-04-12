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
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/hertz/pkg/common/json"
)

func Encode(w io.Writer, e *Event) (err error) {
	err = writeID(w, e.ID)
	if err != nil {
		return
	}
	err = writeEvent(w, e.Event)
	if err != nil {
		return
	}
	err = writeRetry(w, e.Retry)
	if err != nil {
		return
	}
	err = writeData(w, e.Data)
	if err != nil {
		return
	}
	return nil
}

func writeID(w io.Writer, id string) (err error) {
	if len(id) > 0 {
		_, err = w.Write([]byte("id:"))
		if err != nil {
			return
		}
		_, err = fieldReplacer.WriteString(w, id)
		if err != nil {
			return
		}
		_, err = w.Write([]byte("\n"))
		if err != nil {
			return
		}
	}

	return
}

func writeEvent(w io.Writer, event string) (err error) {
	if len(event) > 0 {
		_, err = w.Write([]byte("event:"))
		if err != nil {
			return
		}
		_, err = fieldReplacer.WriteString(w, event)
		if err != nil {
			return
		}

		_, err = w.Write([]byte("\n"))
		if err != nil {
			return
		}
	}

	return
}

func writeRetry(w io.Writer, retry uint) (err error) {
	if retry > 0 {
		_, err = w.Write([]byte("retry:"))
		if err != nil {
			return
		}
		_, err = w.Write(util.Str2Bytes(strconv.FormatUint(uint64(retry), 10)))
		if err != nil {
			return
		}
		_, err = w.Write([]byte("\n"))
		if err != nil {
			return
		}
	}

	return
}

func writeData(w io.Writer, data interface{}) (err error) {
	_, err = w.Write([]byte("data:"))
	if err != nil {
		return err
	}
	switch kindOfData(data) {
	case reflect.Struct, reflect.Slice, reflect.Map:
		err = json.NewEncoder(w).Encode(data)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte("\n"))
		if err != nil {
			return
		}
	default:
		_, err = dataReplacer.WriteString(w, fmt.Sprint(data))
		if err != nil {
			return
		}
		_, err = w.Write([]byte("\n\n"))
		if err != nil {
			return
		}
	}
	return nil
}

func kindOfData(data interface{}) reflect.Kind {
	value := reflect.ValueOf(data)
	valueType := value.Kind()
	if valueType == reflect.Ptr {
		valueType = value.Elem().Kind()
	}
	return valueType
}
