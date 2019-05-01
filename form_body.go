package httpbinder

import (
	"mime"
	"net/http"
)

func BindFormBody(req *http.Request, outPtr interface{}) error {
	// skip GET method
	if req.Method == http.MethodGet {
		return nil
	}

	contentType := req.Header.Get("content-type")
	mediaType, _, _ := mime.ParseMediaType(contentType)

	if mediaType !="application/x-www-form-urlencoded" {
		return nil
	}

	if err := req.ParseForm(); err != nil {
		return err
	}

	return bindValues(req.Form, "form", outPtr)
}