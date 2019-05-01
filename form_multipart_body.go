package httpbinder

import (
	"mime"
	"net/http"
)

func BindMultipartFormBody(maxMemory int64) BindFunc {
	return func(req *http.Request, outPtr interface{}) error {
		// skip GET method
		if req.Method == http.MethodGet {
			return nil
		}

		contentType := req.Header.Get("content-type")
		mediaType, _, _ := mime.ParseMediaType(contentType)

		if mediaType != "multipart/form-data" {
			return nil
		}

		if err := req.ParseMultipartForm(maxMemory); err != nil {
			return nil
		}

		if err := bindValues(req.Form, "form", outPtr); err != nil {
			return err
		}

		// TODO handle binding file to multipart form

		return nil
	}
}
