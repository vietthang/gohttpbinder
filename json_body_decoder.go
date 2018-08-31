package httpbinder

import (
	"io"
	"mime"
	"net/http"
	"encoding/json"
)

type JSONBodyDecoder struct {
}

func (_ JSONBodyDecoder) Match(req *http.Request) bool {
	contentType := req.Header.Get("content-type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}

func (_ JSONBodyDecoder) DecodeBody(body io.Reader, out interface{}) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	return nil
}
