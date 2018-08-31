package httpbinder

import (
	"io"
	"net/http"
)

type BodyDecoder interface {
	Match(req *http.Request) bool
	DecodeBody(body io.Reader, out interface{}) error
}
