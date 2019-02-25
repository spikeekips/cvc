package cvc

import (
	"encoding/json"
	"fmt"
	"sync"
)

const (
	_ = iota + 1
	ErrorMethodNotFoundCode
	ErrorInvalidMethodCode
)

var (
	ErrorMethodNotFound, _ = NewError(ErrorMethodNotFoundCode, "method not found")
	ErrorInvalidMethod, _  = NewError(ErrorInvalidMethodCode, "invalid method found")
)

type Error struct {
	sync.RWMutex
	Code    int
	Message string
	Extra   map[string]interface{}
}

func NewError(code int, message string, extras ...interface{}) (Error, error) {
	if len(extras)%2 != 0 {
		return Error{}, fmt.Errorf("`extras` must be <key>, <value> pair")
	}

	extra := map[string]interface{}{}
	for i := 0; i < len(extras)-1; i = i + 2 {
		extra[extras[i].(string)] = extras[i]
	}

	return Error{
		Code:    code,
		Message: message,
		Extra:   extra,
	}, nil
}

func (e Error) Error() string {
	e.RLock()
	defer e.RUnlock()

	b, _ := json.Marshal(e)
	return string(b)
}

func (e Error) JSON() (o map[string]interface{}) {
	e.RLock()
	defer e.RUnlock()

	b, _ := json.Marshal(e)
	json.Unmarshal(b, &o)

	return
}

func (e *Error) Equal(i error) bool {
	e.RLock()
	defer e.RUnlock()

	o, found := i.(*Error)
	if !found {
		b, found := i.(Error)
		if !found {
			return false
		}
		o = &b
	}

	return e.Code == o.Code
}

func (e Error) Clone() *Error {
	e.RLock()
	defer e.RUnlock()

	extra := map[string]interface{}{}
	for k, v := range e.Extra {
		extra[k] = v
	}

	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Extra:   extra,
	}
}

func (e *Error) Set(k string, v interface{}) *Error {
	e.Lock()
	defer e.Unlock()

	e.Extra[k] = v
	return e
}
