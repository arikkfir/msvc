package http

import (
	"encoding/json"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

type requestDecoder struct {
	targetType reflect.Type
	parsers    []func(*http.Request, reflect.Value) error
}

func newRequestDecoder(targetType reflect.Type) (*requestDecoder, error) {
	if targetType.Kind() != reflect.Struct {
		return nil, errors.Errorf("expected struct for request decoder target type; received '%s'", targetType.Kind())
	}

	parsers := make([]func(*http.Request, reflect.Value) error, 0, 10)
	for i := 0; i < targetType.NumField(); i++ {
		fieldType := targetType.Field(i)

		tag, ok := fieldType.Tag.Lookup("http")
		if !ok {
			return nil, errors.Errorf("missing 'http' tag for field '%s'", fieldType.Name)
		}

		tokens := strings.Split(tag, ",")
		if len(tokens) == 0 || len(tokens) == 1 && strings.TrimSpace(tokens[0]) == "" {
			return nil, errors.Errorf("illegal 'http' tag for field '%s': no tokens", fieldType.Name)
		} else if len(tokens) == 1 && tokens[0] == "body" {
			parsers = append(parsers, newBodyDecoder(fieldType))
		} else if len(tokens) > 2 {
			return nil, errors.Errorf("illegal 'http' tag for field '%s': %s", fieldType.Name, tag)
		} else {
			if len(tokens) == 1 {
				tokens = []string{tokens[0], strings.ToLower(fieldType.Name)}
			}
			switch tokens[0] {
			case "query":
				parsers = append(parsers, newQueryParameterDecoder(fieldType, tokens[1]))
			case "path":
				parsers = append(parsers, newPathParameterDecoder(fieldType, tokens[1]))
			case "header":
				parsers = append(parsers, newHeaderDecoder(fieldType, tokens[1]))
			case "cookie":
				parsers = append(parsers, newCookieDecoder(fieldType, tokens[1]))
			default:
				return nil, errors.Errorf("illegal 'http' tag for field '%s': %s", fieldType.Name, tag)
			}
		}
	}
	return &requestDecoder{targetType, parsers}, nil
}

func (d *requestDecoder) decode(r *http.Request) (interface{}, error) {
	err := r.ParseForm()
	if err != nil {
		panic(errors.Wrapf(err, "failed parsing HTTP request form data"))
	}

	structValuePtr := reflect.New(d.targetType)
	structValue := structValuePtr.Elem()
	for _, parser := range d.parsers {
		if err := parser(r, structValue); err != nil {
			return nil, errors.Wrapf(err, "failed parsing request")
		}
	}

	return structValue.Interface(), nil
}

func newBodyDecoder(field reflect.StructField) func(*http.Request, reflect.Value) error {
	return func(r *http.Request, structValue reflect.Value) error {
		switch contentType := r.Header.Get("content-type"); contentType {
		case "application/json":
			newValuePtr := reflect.New(field.Type)
			if r.ContentLength == 0 {
				structValue.FieldByIndex(field.Index).Set(reflect.Zero(field.Type))
			} else {
				jsonDecoder := json.NewDecoder(r.Body)
				jsonDecoder.DisallowUnknownFields()
				if err := jsonDecoder.Decode(newValuePtr.Interface()); err != nil {
					return errors.Wrapf(err, "failed reading JSON into '%s'", field.Type.Name())
				}
				structValue.FieldByIndex(field.Index).Set(newValuePtr.Elem())
			}
			return nil
		default:
			return NewHttpError(http.StatusUnsupportedMediaType, errors.Errorf("'%s' is not supported", contentType))
		}
	}
}

func newQueryParameterDecoder(field reflect.StructField, name string) func(*http.Request, reflect.Value) error {
	var inject func([]string, reflect.Value) error

	inject = func(values []string, targetValue reflect.Value) error {
		switch targetValue.Type().Kind() {
		case reflect.Ptr:
			if values != nil {
				ptrType := reflect.PtrTo(targetValue.Type().Elem())
				ptrValue := reflect.New(ptrType.Elem())
				err := inject(values, ptrValue.Elem())
				if err != nil {
					return err
				} else {
					targetValue.Set(ptrValue)
					return nil
				}
			} else {
				targetValue.Set(reflect.Zero(targetValue.Type()))
				return nil
			}
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String:
			if values != nil {
				if len(values) > 0 {
					return injectScalarValue(values[0], targetValue)
				} else {
					targetValue.Set(reflect.Zero(targetValue.Type()))
					return nil
				}
			} else {
				targetValue.Set(reflect.Zero(targetValue.Type()))
				return nil
			}
		case reflect.Slice:
			if values != nil {
				sliceItemType := targetValue.Type().Elem()
				sliceValue := reflect.MakeSlice(targetValue.Type(), len(values), cap(values))
				for i, v := range values {
					sliceItemPtrValue := reflect.New(sliceItemType)
					err := inject([]string{v}, sliceItemPtrValue.Elem())
					if err != nil {
						return err
					}
					sliceValue.Index(i).Set(sliceItemPtrValue.Elem())
				}
				targetValue.Set(sliceValue)
				return nil
			} else {
				targetValue.Set(reflect.Zero(targetValue.Type()))
				return nil
			}
		default:
			panic(errors.Errorf("injecting query parameters into fields of type '%s' is not supported (field '%s')", field.Type.Kind(), field.Name))
		}
	}
	return func(r *http.Request, structValue reflect.Value) error {
		values, ok := r.Form[name]
		if !ok {
			values = nil
		}
		return inject(values, structValue.FieldByIndex(field.Index))
	}
}

func newPathParameterDecoder(field reflect.StructField, name string) func(*http.Request, reflect.Value) error {
	var inject func(string, reflect.Value) error

	inject = func(value string, targetValue reflect.Value) error {
		switch targetValue.Type().Kind() {
		case reflect.Ptr:
			ptrType := reflect.PtrTo(targetValue.Type().Elem())
			ptrValue := reflect.New(ptrType.Elem())
			if err := inject(value, ptrValue.Elem()); err != nil {
				return err
			} else {
				targetValue.Set(ptrValue)
				return nil
			}
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String:
			return injectScalarValue(value, targetValue)
		default:
			panic(errors.Errorf("injecting path parameters into fields of type '%s' is not supported (field '%s')", field.Type.Kind(), field.Name))
		}
	}
	return func(r *http.Request, structValue reflect.Value) error {
		value := chi.URLParam(r, name)
		if value == "" {
			return errors.Errorf("empty value for path parameter '%s', required for field '%s'", name, field.Name)
		} else {
			return inject(value, structValue.FieldByIndex(field.Index))
		}
	}
}

func newHeaderDecoder(field reflect.StructField, name string) func(*http.Request, reflect.Value) error {
	var inject func([]string, reflect.Value) error

	inject = func(values []string, targetValue reflect.Value) error {
		switch targetValue.Type().Kind() {
		case reflect.Ptr:
			if values != nil {
				ptrType := reflect.PtrTo(targetValue.Type().Elem())
				ptrValue := reflect.New(ptrType.Elem())
				err := inject(values, ptrValue.Elem())
				if err != nil {
					return err
				} else {
					targetValue.Set(ptrValue)
					return nil
				}
			} else {
				targetValue.Set(reflect.Zero(targetValue.Type()))
				return nil
			}
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String:
			if values != nil {
				if len(values) > 0 {
					return injectScalarValue(values[0], targetValue)
				} else {
					targetValue.Set(reflect.Zero(targetValue.Type()))
					return nil
				}
			} else {
				targetValue.Set(reflect.Zero(targetValue.Type()))
				return nil
			}
		case reflect.Slice:
			if values != nil {
				sliceItemType := targetValue.Type().Elem()
				sliceValue := reflect.MakeSlice(targetValue.Type(), len(values), cap(values))
				for i, v := range values {
					sliceItemPtrValue := reflect.New(sliceItemType)
					err := inject([]string{v}, sliceItemPtrValue.Elem())
					if err != nil {
						return err
					}
					sliceValue.Index(i).Set(sliceItemPtrValue.Elem())
				}
				targetValue.Set(sliceValue)
				return nil
			} else {
				targetValue.Set(reflect.Zero(targetValue.Type()))
				return nil
			}
		default:
			panic(errors.Errorf("injecting headers into fields of type '%s' is not supported (field '%s')", field.Type.Kind(), field.Name))
		}
	}
	return func(r *http.Request, structValue reflect.Value) error {
		values, ok := r.Header[http.CanonicalHeaderKey(name)]
		if !ok {
			values = nil
		}
		return inject(values, structValue.FieldByIndex(field.Index))
	}
}

func newCookieDecoder(field reflect.StructField, name string) func(*http.Request, reflect.Value) error {
	var inject func(*string, reflect.Value) error

	inject = func(value *string, targetValue reflect.Value) error {
		switch targetValue.Type().Kind() {
		case reflect.Ptr:
			if value != nil {
				ptrType := reflect.PtrTo(targetValue.Type().Elem())
				ptrValue := reflect.New(ptrType.Elem())
				err := inject(value, ptrValue.Elem())
				if err != nil {
					return err
				} else {
					targetValue.Set(ptrValue)
					return nil
				}
			} else {
				targetValue.Set(reflect.Zero(targetValue.Type()))
				return nil
			}
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String:
			if value != nil {
				return injectScalarValue(*value, targetValue)
			} else {
				targetValue.Set(reflect.Zero(targetValue.Type()))
				return nil
			}
		default:
			panic(errors.Errorf("injecting cookie values into fields of type '%s' is not supported (field '%s')", field.Type.Kind(), field.Name))
		}
	}
	return func(r *http.Request, structValue reflect.Value) error {
		var cookieValue *string = nil
		cookie, err := r.Cookie(name)
		if err != nil {
			if err == http.ErrNoCookie {
				cookieValue = nil
			} else {
				return err
			}
		} else {
			cookieValue = &cookie.Value
		}
		return inject(cookieValue, structValue.FieldByIndex(field.Index))
	}
}

func injectScalarValue(stringValue string, targetValue reflect.Value) error {
	switch kind := targetValue.Kind(); kind {
	case reflect.Bool:
		if b, err := strconv.ParseBool(stringValue); err != nil {
			return err
		} else {
			targetValue.SetBool(b)
		}
	case reflect.Int:
		if i, err := strconv.ParseInt(stringValue, 0, 0); err != nil {
			return err
		} else {
			targetValue.SetInt(i)
		}
	case reflect.Int8:
		if i, err := strconv.ParseInt(stringValue, 0, 8); err != nil {
			return err
		} else {
			targetValue.SetInt(i)
		}
	case reflect.Int16:
		if i, err := strconv.ParseInt(stringValue, 0, 16); err != nil {
			return err
		} else {
			targetValue.SetInt(i)
		}
	case reflect.Int32:
		if i, err := strconv.ParseInt(stringValue, 0, 32); err != nil {
			return err
		} else {
			targetValue.SetInt(i)
		}
	case reflect.Int64:
		if i, err := strconv.ParseInt(stringValue, 0, 64); err != nil {
			return err
		} else {
			targetValue.SetInt(i)
		}
	case reflect.Uint:
		if i, err := strconv.ParseUint(stringValue, 0, 0); err != nil {
			return err
		} else {
			targetValue.SetUint(i)
		}
	case reflect.Uint8:
		if i, err := strconv.ParseUint(stringValue, 0, 8); err != nil {
			return err
		} else {
			targetValue.SetUint(i)
		}
	case reflect.Uint16:
		if i, err := strconv.ParseUint(stringValue, 0, 16); err != nil {
			return err
		} else {
			targetValue.SetUint(i)
		}
	case reflect.Uint32:
		if i, err := strconv.ParseUint(stringValue, 0, 32); err != nil {
			return err
		} else {
			targetValue.SetUint(i)
		}
	case reflect.Uint64:
		if i, err := strconv.ParseUint(stringValue, 0, 64); err != nil {
			return err
		} else {
			targetValue.SetUint(i)
		}
	case reflect.Float32:
		if f, err := strconv.ParseFloat(stringValue, 32); err != nil {
			return err
		} else {
			targetValue.SetFloat(f)
		}
	case reflect.Float64:
		if f, err := strconv.ParseFloat(stringValue, 64); err != nil {
			return err
		} else {
			targetValue.SetFloat(f)
		}
	case reflect.String:
		targetValue.SetString(stringValue)
	default:
		panic(errors.Errorf("unsupported field type: %s", kind))
	}
	return nil
}
