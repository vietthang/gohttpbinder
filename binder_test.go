package httpbinder

import (
	"bytes"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestBindRequestBasic(t *testing.T) {
	assert := require.New(t)

	type input struct {
		Q        string  `query:"q"`
		Foo      *string `query:"foo"`
		Fooz     *string `query:"fooz"`
		IntVal   int     `query:"int_val"`
		FloatVal float32 `query:"float_val"`
		UintVal  uint    `query:"uint_val"`
		BoolVal  bool    `query:"bool_val"`
		Binary   []byte  `query:"binary_val"`
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.com/people?q=a&foo=bar&int_val=1&float_val=1.2&uint_val=1&bool_val=true&binary_val=YWJj", nil)
	assert.Nil(err)

	i := &input{}
	err = DefaultBinding(req, i)
	assert.Nil(err)

	bar := "bar"

	assert.Equal(i, &input{
		Q:        "a",
		Foo:      &bar,
		Fooz:     nil,
		IntVal:   1,
		FloatVal: 1.2,
		UintVal:  1,
		BoolVal:  true,
		Binary:   []byte("abc"),
	})
}

func TestBindHeader(t *testing.T) {
	assert := require.New(t)

	type input struct {
		ContentType string `header:"content-type"`
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.com/people/1", nil)
	assert.Nil(err)
	req.Header.Set("Content-Type", "application/json")

	i := &input{}
	err = DefaultBinding(req, i)
	assert.Nil(err)

	assert.Equal(i, &input{
		ContentType: "application/json",
	})
}

func TestBindRequestWithParamExtractor(t *testing.T) {
	assert := require.New(t)

	type input struct {
		Q        string `query:"q"`
		PersonID int64  `param:"person_id"`
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.com/people/1?q=a&foo=bar", nil)
	assert.Nil(err)

	i := &input{}
	binding := Compose(
		DefaultBinding,
		BindParam(func(req *http.Request, name string) string {
			if name == "person_id" {
				return "1"
			}
			return ""
		}),
	)

	err = binding(req, i)
	assert.Nil(err)

	assert.Equal(i, &input{
		Q:        "a",
		PersonID: 1,
	})
}

type HexInt64 int64

func (int HexInt64) MarshalText() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(int), 16)), nil
}

func (int *HexInt64) UnmarshalText(text []byte) error {
	val, err := strconv.ParseInt(string(text), 16, 64)
	if err != nil {
		return err
	}
	*int = HexInt64(val)
	return nil
}

func TestBindRequestWithTextUnmarshaller(t *testing.T) {
	assert := require.New(t)

	type input struct {
		SenderID         HexInt64  `query:"sender_id"`
		MaybeRecipientID *HexInt64 `query:"recipient_id"`
		AuditorID        *HexInt64 `query:"auditor_id"`
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.com/people?sender_id=aa&recipient_id=bb", nil)
	assert.Nil(err)

	i := &input{}
	err = DefaultBinding(req, i)
	assert.Nil(err)

	var recipientID HexInt64 = 0xbb

	assert.Equal(i, &input{
		SenderID:         0xaa,
		MaybeRecipientID: &recipientID,
		AuditorID:        nil,
	})
}

func TestBindRequestWithTextUnmarshallerError(t *testing.T) {
	assert := require.New(t)

	type input struct {
		SenderID         HexInt64  `query:"sender_id"`
		MaybeRecipientID *HexInt64 `query:"recipient_id"`
		AuditorID        *HexInt64 `query:"auditor_id"`
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.com/people?sender_id=zzz&recipient_id=bb", nil)
	assert.Nil(err)

	i := &input{}
	err = DefaultBinding(req, i)
	assert.Error(err)
}

func TestBindRequestWithSlice(t *testing.T) {
	type input struct {
		HexInt64Slice    []HexInt64  `query:"int_slice"`
		HexInt64PtrSlice []*HexInt64 `query:"int_ptr_slice"`
		StringSlice      []string    `query:"string_slice"`
	}

	t.Parallel()

	t.Run("with int_slice", func(t *testing.T) {
		assert := require.New(t)

		req, err := http.NewRequest(http.MethodGet, "http://example.com?int_slice=aa&int_slice=bb", nil)
		assert.Nil(err)

		i := &input{}
		err = DefaultBinding(req, i)

		assert.Equal(i, &input{
			HexInt64Slice:    []HexInt64{0xaa, 0xbb},
			HexInt64PtrSlice: make([]*HexInt64, 0),
			StringSlice:      make([]string, 0),
		})
	})

	t.Run("with int_ptr_slice", func(t *testing.T) {
		assert := require.New(t)

		req, err := http.NewRequest(http.MethodGet, "http://example.com?int_ptr_slice=aa&int_ptr_slice=bb", nil)
		assert.Nil(err)

		i := &input{}
		err = DefaultBinding(req, i)

		var aa HexInt64 = 0xaa
		var bb HexInt64 = 0xbb

		assert.Equal(i, &input{
			HexInt64Slice:    make([]HexInt64, 0),
			HexInt64PtrSlice: []*HexInt64{&aa, &bb},
			StringSlice:      make([]string, 0),
		})
	})

	t.Run("with string_slice", func(t *testing.T) {
		assert := require.New(t)

		req, err := http.NewRequest(http.MethodGet, "http://example.com?string_slice=aa&string_slice=bb", nil)
		assert.Nil(err)

		i := &input{}
		err = DefaultBinding(req, i)

		assert.Equal(i, &input{
			HexInt64Slice:    make([]HexInt64, 0),
			HexInt64PtrSlice: make([]*HexInt64, 0),
			StringSlice:      []string{"aa", "bb"},
		})
	})
}

func TestBindRequestWithInvalidKind(t *testing.T) {
	assert := require.New(t)

	type input struct {
		Q struct{} `query:"q"`
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.com?q=a", nil)
	assert.Nil(err)

	i := &input{}
	err = DefaultBinding(req, i)
	assert.Error(err)
}

func TestBindRequestWithJSONBody(t *testing.T) {
	assert := require.New(t)

	type body struct {
		Foo string `json:"foo"`
	}

	bodyJSON, err := json.Marshal(body{Foo: "bar"})
	assert.Nil(err)

	req, err := http.NewRequest(http.MethodPost, "http://example.com?q=a", bytes.NewReader(bodyJSON))
	assert.Nil(err)
	req.Header.Set("content-type", "application/json; charset=utf-8")

	type input struct {
		Q string `query:"q"`
		body
	}

	i := &input{}
	err = DefaultBinding(req, i)
	assert.Nil(err)

	assert.Equal(i, &input{
		Q: "a",
		body: body{
			Foo: "bar",
		},
	})
}

func TestBindRequestWithInvalidJSONBody(t *testing.T) {
	assert := require.New(t)

	type body struct {
		Foo string `json:"foo"`
	}

	req, err := http.NewRequest(http.MethodPost, "http://example.com?q=a", strings.NewReader("foo"))
	assert.Nil(err)
	req.Header.Set("content-type", "application/json; charset=utf-8")

	type input struct {
		Q string `query:"q"`
		body
	}

	i := &input{}
	err = DefaultBinding(req, i)
	assert.Error(err)
}

func TestBindRequestWithValidator(t *testing.T) {
	type input struct {
		Q string `query:"q"`
	}

	binding := Compose(
		DefaultBinding,
		func(_ *http.Request, outPtr interface{}) error {
			if outPtr.(*input).Q == "a" {
				return nil
			}
			return errors.New("validation error")
		},
	)

	t.Parallel()

	t.Run("With valid input", func(t *testing.T) {
		assert := require.New(t)

		req, err := http.NewRequest(http.MethodGet, "http://example.com?q=a", nil)
		assert.Nil(err)

		i := &input{}
		err = binding(req, i)
		assert.Nil(err)
	})

	t.Run("With invalid input", func(t *testing.T) {
		assert := require.New(t)

		req, err := http.NewRequest(http.MethodGet, "http://example.com?q=b", nil)
		assert.Nil(err)

		i := &input{}
		err = binding(req, i)
		assert.Error(err, "validation error")
	})
}

func TestBindRequestWithFormBody(t *testing.T) {
	assert := require.New(t)

	form := url.Values{}
	form.Add("formField", "bar")
	req, err := http.NewRequest(http.MethodPost, "http://example.com?q=a", strings.NewReader(form.Encode()))
	assert.Nil(err)

	req.Header.Set("content-type", "application/x-www-form-urlencoded; charset=utf-8")

	type input struct {
		Q         string `query:"q"`
		FormField string `form:"formField"`
		JSONField string `json:"jsonField"`
	}

	i := &input{}
	err = DefaultBinding(req, i)
	assert.Nil(err)

	assert.Equal(i, &input{
		Q:         "a",
		FormField: "bar",
		JSONField: "",
	})
}

func TestBindRequestWithMultipartFormBody(t *testing.T) {
	assert := require.New(t)

	buffer := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buffer)
	err := writer.WriteField("formField", "bar")
	assert.Nil(err)
	err = writer.Close()
	assert.Nil(err)

	req, err := http.NewRequest(http.MethodPost, "http://example.com?q=a", buffer)
	assert.Nil(err)

	req.Header.Set("content-type", writer.FormDataContentType())

	type input struct {
		Q         string `query:"q"`
		FormField string `form:"formField"`
		JSONField string `json:"jsonField"`
	}

	i := &input{}
	err = DefaultBinding(req, i)
	assert.Nil(err)

	assert.Equal(i, &input{
		Q:         "a",
		FormField: "bar",
		JSONField: "",
	})
}
