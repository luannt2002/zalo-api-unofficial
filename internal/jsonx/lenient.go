package jsonx

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

// UnmarshalLenient decodes like json.Unmarshal, but when it hits a scalar of the
// other kind — a number where the Go field is a string, or a numeric string
// where it is a number — it converts that scalar and decodes again. Zalo sends
// the same field either way; the JS SDK never notices, a strict Go decode loses
// the whole event. Types with their own UnmarshalJSON are left to themselves.
func UnmarshalLenient(data []byte, v any) error {
	err := json.Unmarshal(data, v)
	var te *json.UnmarshalTypeError
	if err == nil || !errors.As(err, &te) {
		return err
	}

	var tree any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // keep 19-digit ids exact
	if dec.Decode(&tree) != nil {
		return err
	}
	fixed, merr := json.Marshal(coerce(tree, reflect.TypeOf(v)))
	if merr != nil {
		return err
	}
	return json.Unmarshal(fixed, v)
}

// ScalarString returns a JSON string's value, or a JSON number's literal text;
// "" for null or absent values.
func ScalarString(b json.RawMessage) string {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		return s
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return ""
	}
	return string(b)
}

var unmarshalerType = reflect.TypeFor[json.Unmarshaler]()

func coerce(v any, t reflect.Type) any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if reflect.PointerTo(t).Implements(unmarshalerType) {
		return v
	}

	switch t.Kind() {
	case reflect.String:
		if n, ok := v.(json.Number); ok {
			return n.String()
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		if s, ok := v.(string); ok {
			s = strings.TrimSpace(s)
			if s == "" {
				return json.Number("0") // JS Number("") == 0
			}
			if (s[0] == '-' || (s[0] >= '0' && s[0] <= '9')) && json.Valid([]byte(s)) {
				return json.Number(s)
			}
		}
	case reflect.Slice, reflect.Array:
		if arr, ok := v.([]any); ok {
			for i := range arr {
				arr[i] = coerce(arr[i], t.Elem())
			}
		}
	case reflect.Map:
		if obj, ok := v.(map[string]any); ok {
			for k, val := range obj {
				obj[k] = coerce(val, t.Elem())
			}
		}
	case reflect.Struct:
		if obj, ok := v.(map[string]any); ok {
			fields := jsonFields(t)
			for k, val := range obj {
				ft, ok := fields[k]
				if !ok {
					for name, f := range fields { // encoding/json matches keys case-insensitively
						if strings.EqualFold(name, k) {
							ft, ok = f, true
							break
						}
					}
				}
				if ok {
					obj[k] = coerce(val, ft)
				}
			}
		}
	}
	return v
}

// jsonFields maps JSON names to field types; embedded structs are flattened and
// shallower fields win, as in encoding/json.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	var embedded []reflect.Type
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if f.Anonymous && name == "" {
			et := f.Type
			if et.Kind() == reflect.Pointer {
				et = et.Elem()
			}
			if et.Kind() == reflect.Struct {
				embedded = append(embedded, et)
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out[name] = f.Type
	}
	for _, et := range embedded {
		for k, ft := range jsonFields(et) {
			if _, ok := out[k]; !ok {
				out[k] = ft
			}
		}
	}
	return out
}
