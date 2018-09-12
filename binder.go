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

type BindFunc func(req *http.Request, outPtr interface{}) error

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

func BindQuery(req *http.Request, outPtr interface{}) error {
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

func BindHeader(req *http.Request, outPtr interface{}) error {
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

func BindParam(paramExtractor ParamExtractor) BindFunc {
	return func(req *http.Request, outPtr interface{}) error {
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
}

func Compose(fns ...BindFunc) BindFunc {
	return func(req *http.Request, outPtr interface{}) error {
		for _, fn := range fns {
			if err := fn(req, outPtr); err != nil {
				return err
			}
		}
		return nil
	}
}

var DefaultBinding = Compose(
	BindHeader,
	BindQuery,
	BindJSONBody,
)
