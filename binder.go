package httpbinder

import (
	"net/http"
	"reflect"
	"errors"
	"strconv"
	"encoding"
	"encoding/base64"
	"net/textproto"
)

type ParamExtractor func(req *http.Request, name string) string

type Validator func(in interface{}) error

type Option func(*bindOptions)

type bindOptions struct {
	paramExtractor ParamExtractor
	validator      Validator
	bodyDecoders   []BodyDecoder
}

func WithParamExtractor(paramExtractor ParamExtractor) Option {
	return func(options *bindOptions) {
		options.paramExtractor = paramExtractor
	}
}

func WithValidator(validator Validator) Option {
	return func(options *bindOptions) {
		options.validator = validator
	}
}

func WithBodyDecoder(decoder BodyDecoder) Option {
	return func(options *bindOptions) {
		options.bodyDecoders = append(options.bodyDecoders, decoder)
	}
}

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func bind(outValue reflect.Value, values []string) error {
	outType := outValue.Type()

	if outType.AssignableTo(textUnmarshalerType) {
		if outType.Kind() != reflect.Ptr {
			return errors.New("text unmarshaler type is not a pointer")
		}
		if len(values) == 0 || values[0] == "" {
			return nil
		}
		newFieldValue := reflect.New(outType.Elem())
		if err := newFieldValue.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(values[0])); err != nil {
			return err
		}
		outValue.Set(newFieldValue)
		return nil
	}

	if reflect.PtrTo(outType).AssignableTo(textUnmarshalerType) {
		if len(values) == 0 || values[0] == "" {
			return nil
		}
		newFieldValue := reflect.New(outType)
		if err := newFieldValue.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(values[0])); err != nil {
			return err
		}
		outValue.Set(newFieldValue.Elem())
		return nil
	}

	if outType == reflect.TypeOf((*[]byte)(nil)).Elem() {
		valueBytes, err := base64.StdEncoding.DecodeString(values[0])
		if err != nil {
			return err
		}
		outValue.Set(reflect.ValueOf(valueBytes))
		return nil
	}

	switch outType.Kind() {
	case reflect.String:
		if len(values) == 0 || values[0] == "" {
			return nil
		}
		outValue.SetString(values[0])
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if len(values) == 0 || values[0] == "" {
			return nil
		}
		intVal, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil {
			return err
		}
		outValue.SetInt(intVal)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if len(values) == 0 || values[0] == "" {
			return nil
		}
		uintVal, err := strconv.ParseUint(values[0], 10, 64)
		if err != nil {
			return err
		}
		outValue.SetUint(uintVal)
		return nil
	case reflect.Bool:
		if len(values) == 0 || values[0] == "" {
			return nil
		}
		boolVal, err := strconv.ParseBool(values[0])
		if err != nil {
			return err
		}
		outValue.SetBool(boolVal)
		return nil
	case reflect.Float32, reflect.Float64:
		if len(values) == 0 || values[0] == "" {
			return nil
		}
		floatVal, err := strconv.ParseFloat(values[0], 64)
		if err != nil {
			return err
		}
		outValue.SetFloat(floatVal)
		return nil
	case reflect.Ptr:
		if len(values) == 0 || values[0] == "" {
			return nil
		}
		newValue := reflect.New(outType.Elem())
		if err := bind(newValue.Elem(), values); err != nil {
			return err
		}
		outValue.Set(newValue)
		return nil
	case reflect.Slice:
		sliceValue := reflect.MakeSlice(outType, len(values), len(values))
		for index, value := range values {
			elemValue := reflect.New(outType.Elem()).Elem()
			if err := bind(elemValue, []string{value}); err != nil {
				return err
			}
			sliceValue.Index(index).Set(elemValue)
		}
		outValue.Set(sliceValue)
		return nil
	default:
		return errors.New("invalid kind")
	}
}

func bindQuery(req *http.Request, outPtr interface{}) error {
	outPtrValue := reflect.ValueOf(outPtr)
	if outPtrValue.Kind() != reflect.Ptr {
		return errors.New("out is not a pointer")
	}
	outValue := outPtrValue.Elem()
	outType := outValue.Type()
	for i := 0; i < outType.NumField(); i++ {
		field := outType.Field(i)
		queryTag := field.Tag.Get("query")
		if queryTag == "" {
			continue
		}

		urlQuery := req.URL.Query()
		outFieldValue := outValue.Field(i)

		if err := bind(outFieldValue, urlQuery[queryTag]); err != nil {
			return err
		}
	}

	return nil
}

func bindHeader(req *http.Request, outPtr interface{}) error {
	outPtrValue := reflect.ValueOf(outPtr)
	if outPtrValue.Kind() != reflect.Ptr {
		return errors.New("out is not a pointer")
	}
	outValue := outPtrValue.Elem()
	outType := outValue.Type()
	for i := 0; i < outType.NumField(); i++ {
		field := outType.Field(i)
		headerTag := field.Tag.Get("header")
		if headerTag == "" {
			continue
		}

		outFieldValue := outValue.Field(i)

		if err := bind(outFieldValue, req.Header[textproto.CanonicalMIMEHeaderKey(headerTag)]); err != nil {
			return err
		}
	}

	return nil
}

func bindParam(req *http.Request, paramExtractor ParamExtractor, outPtr interface{}) error {
	outPtrValue := reflect.ValueOf(outPtr)
	if outPtrValue.Kind() != reflect.Ptr {
		return errors.New("out is not a pointer")
	}
	outValue := outPtrValue.Elem()
	outType := outValue.Type()
	for i := 0; i < outType.NumField(); i++ {
		field := outType.Field(i)
		paramTag := field.Tag.Get("param")
		if paramTag == "" {
			continue
		}

		paramValue := paramExtractor(req, paramTag)
		outFieldValue := outValue.Field(i)

		if err := bind(outFieldValue, []string{paramValue}); err != nil {
			return err
		}
	}

	return nil
}

type Binder struct {
	options *bindOptions
}

func NewBinder(options ...Option) *Binder {
	bindOptions := &bindOptions{}
	for _, option := range options {
		option(bindOptions)
	}

	return &Binder{
		options: bindOptions,
	}
}

func (binder *Binder) BindRequest(req *http.Request, out interface{}) error {
	if req.Method != http.MethodGet {
		defer req.Body.Close()

		for _, decoder := range binder.options.bodyDecoders {
			if !decoder.Match(req) {
				continue
			}

			if err := decoder.DecodeBody(req.Body, out); err != nil {
				return err
			}
			break
		}
	}

	if err := bindQuery(req, out); err != nil {
		return err
	}

	if err := bindHeader(req, out); err != nil {
		return err
	}

	if binder.options.paramExtractor != nil {
		if err := bindParam(req, binder.options.paramExtractor, out); err != nil {
			return err
		}
	}

	if binder.options.validator != nil {
		if err := binder.options.validator(out); err != nil {
			return err
		}
	}

	return nil
}
