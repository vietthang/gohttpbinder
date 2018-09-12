package httpbinder

import (
	"mime"
	"net/http"
	"encoding/json"
)

func BindJSONBody(req *http.Request, outPtr interface{}) error {
	// skip GET method
	if req.Method == http.MethodGet {
		return nil
	}

	contentType := req.Header.Get("content-type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	// skip all request with media type is not application/json
	if mediaType != "application/json" {
		return nil
	}

	decoder := json.NewDecoder(req.Body)
	if req.Body != nil {
		defer req.Body.Close()
	}
	if err := decoder.Decode(outPtr); err != nil {
		return err
	}
	return nil
}
