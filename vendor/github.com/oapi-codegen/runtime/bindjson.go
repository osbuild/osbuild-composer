package runtime

import (
	"encoding"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// validatedBody is the interface kin-openapi's openapi3filter gives a request
// body when its Options.KeepValidatedValue is set: the body carries the value
// that was decoded from the request and validated against the schema.
type validatedBody interface {
	ValidatedValue() any
}

// BindJSONBody decodes the JSON request body of r into v, which must be a
// non-nil pointer.
//
// When a request validation middleware built on kin-openapi's openapi3filter
// has run on r with Options.KeepValidatedValue set, r.Body carries the value
// the validator decoded and validated. That value is bound into v with
// BindJSONValue, so the handler receives exactly what was validated: member
// names are matched exactly, where encoding/json matches them
// case-insensitively, and the body is not decoded a second time.
//
// Without a validated value, or when the value cannot be bound into v, the
// body is decoded with encoding/json exactly as before, so error behaviour
// (including io.EOF for an empty body) is unchanged.
func BindJSONBody(r *http.Request, v any) error {
	if r != nil && r.Body != nil {
		if b, ok := r.Body.(validatedBody); ok {
			if err := BindJSONValue(b.ValidatedValue(), v); err == nil {
				return nil
			}
		}
	}
	return json.NewDecoder(r.Body).Decode(v)
}

// BindJSONValue binds a decoded JSON value into v, which must be a non-nil
// pointer. value is the generic representation produced by decoding JSON into
// an `any`: nil, bool, string, json.Number (or a Go numeric type), []any and
// map[string]any.
//
// Binding follows encoding/json's rules for the destination type (struct tags,
// embedded struct promotion, json.Unmarshaler and encoding.TextUnmarshaler,
// base64 for []byte, float64 for `any`), with one deliberate difference:
// object member names are matched to struct fields exactly, never
// case-insensitively. Unknown members are ignored.
//
// On success *v holds only what value describes; on error *v is left
// unchanged. An error means the value does not fit the destination (a type
// mismatch, an overflow, or a destination type that is not supported) and
// the caller should fall back to decoding the original JSON.
func BindJSONValue(value any, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("BindJSONValue: destination must be a non-nil pointer, got %T", v)
	}
	// Bind into a fresh value so a failure part-way through leaves the
	// caller's destination untouched.
	tmp := reflect.New(rv.Type().Elem()).Elem()
	if err := bindJSONValue(value, tmp); err != nil {
		return err
	}
	rv.Elem().Set(tmp)
	return nil
}

var (
	jsonUnmarshalerType = reflect.TypeFor[json.Unmarshaler]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

// errUnsupportedJSONDestination is returned for destination types the binder
// does not handle; BindJSONBody then falls back to encoding/json.
var errUnsupportedJSONDestination = errors.New("unsupported destination type")

func bindJSONValue(value any, dst reflect.Value) error {
	// Pointers are dereferenced first, so that a json.Unmarshaler implemented
	// on the pointee's pointer receiver is found below on an allocated value.
	if dst.Kind() == reflect.Pointer {
		if value == nil {
			dst.SetZero()
			return nil
		}
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		return bindJSONValue(value, dst.Elem())
	}

	if value == nil {
		return bindJSONNull(dst)
	}

	if u := jsonUnmarshalerOf(dst); u != nil {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return u.UnmarshalJSON(data)
	}
	if s, ok := value.(string); ok {
		if u := textUnmarshalerOf(dst); u != nil {
			return u.UnmarshalText([]byte(s))
		}
	}

	switch dst.Kind() {
	case reflect.Interface:
		if dst.NumMethod() != 0 {
			return fmt.Errorf("%w: %s", errUnsupportedJSONDestination, dst.Type())
		}
		conv, err := jsonValueToInterface(value)
		if err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(&conv).Elem())
		return nil

	case reflect.Struct:
		obj, ok := value.(map[string]any)
		if !ok {
			return jsonTypeError(value, dst.Type())
		}
		fields := cachedJSONFields(dst.Type())
		for name, member := range obj {
			f, ok := fields[name]
			if !ok {
				continue
			}
			fv, err := jsonFieldByIndex(dst, f.index)
			if err != nil {
				return err
			}
			if f.quoted {
				if err := bindQuotedJSONValue(member, fv); err != nil {
					return err
				}
				continue
			}
			if err := bindJSONValue(member, fv); err != nil {
				return err
			}
		}
		return nil

	case reflect.Map:
		obj, ok := value.(map[string]any)
		if !ok {
			return jsonTypeError(value, dst.Type())
		}
		t := dst.Type()
		if dst.IsNil() {
			dst.Set(reflect.MakeMapWithSize(t, len(obj)))
		}
		for name, member := range obj {
			key, err := jsonMapKey(name, t.Key())
			if err != nil {
				return err
			}
			elem := reflect.New(t.Elem()).Elem()
			if err := bindJSONValue(member, elem); err != nil {
				return err
			}
			dst.SetMapIndex(key, elem)
		}
		return nil

	case reflect.Slice:
		if s, ok := value.(string); ok && dst.Type().Elem().Kind() == reflect.Uint8 {
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return err
			}
			dst.SetBytes(b)
			return nil
		}
		arr, ok := value.([]any)
		if !ok {
			return jsonTypeError(value, dst.Type())
		}
		slice := reflect.MakeSlice(dst.Type(), len(arr), len(arr))
		for i, item := range arr {
			if err := bindJSONValue(item, slice.Index(i)); err != nil {
				return err
			}
		}
		dst.Set(slice)
		return nil

	case reflect.Array:
		arr, ok := value.([]any)
		if !ok {
			return jsonTypeError(value, dst.Type())
		}
		n := min(len(arr), dst.Len())
		for i := 0; i < n; i++ {
			if err := bindJSONValue(arr[i], dst.Index(i)); err != nil {
				return err
			}
		}
		for i := n; i < dst.Len(); i++ {
			dst.Index(i).SetZero()
		}
		return nil

	case reflect.Bool:
		b, ok := value.(bool)
		if !ok {
			return jsonTypeError(value, dst.Type())
		}
		dst.SetBool(b)
		return nil

	case reflect.String:
		s, ok := value.(string)
		if !ok {
			return jsonTypeError(value, dst.Type())
		}
		dst.SetString(s)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		num, ok := jsonNumberOf(value)
		if !ok {
			return jsonTypeError(value, dst.Type())
		}
		n, err := strconv.ParseInt(string(num), 10, 64)
		if err != nil || dst.OverflowInt(n) {
			return jsonTypeError(value, dst.Type())
		}
		dst.SetInt(n)
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		num, ok := jsonNumberOf(value)
		if !ok {
			return jsonTypeError(value, dst.Type())
		}
		n, err := strconv.ParseUint(string(num), 10, 64)
		if err != nil || dst.OverflowUint(n) {
			return jsonTypeError(value, dst.Type())
		}
		dst.SetUint(n)
		return nil

	case reflect.Float32, reflect.Float64:
		num, ok := jsonNumberOf(value)
		if !ok {
			return jsonTypeError(value, dst.Type())
		}
		f, err := strconv.ParseFloat(string(num), dst.Type().Bits())
		if err != nil {
			return jsonTypeError(value, dst.Type())
		}
		dst.SetFloat(f)
		return nil
	}

	return fmt.Errorf("%w: %s", errUnsupportedJSONDestination, dst.Type())
}

// bindJSONNull applies a JSON null to a non-pointer destination the way
// encoding/json does: a json.Unmarshaler is handed the literal, nil-able kinds
// are set to nil, and anything else is left untouched. (A nil pointer is set
// to nil by the caller without consulting its Unmarshaler, also as
// encoding/json does.)
func bindJSONNull(dst reflect.Value) error {
	if u := jsonUnmarshalerOf(dst); u != nil {
		return u.UnmarshalJSON([]byte("null"))
	}
	switch dst.Kind() {
	case reflect.Interface, reflect.Map, reflect.Slice:
		dst.SetZero()
	}
	return nil
}

// bindQuotedJSONValue handles a field with the `,string` tag option: the JSON
// value is a string whose contents are the JSON literal to decode.
func bindQuotedJSONValue(value any, dst reflect.Value) error {
	if value == nil {
		return bindJSONNull(dst)
	}
	s, ok := value.(string)
	if !ok || !dst.CanAddr() {
		return jsonTypeError(value, dst.Type())
	}
	return json.Unmarshal([]byte(s), dst.Addr().Interface())
}

func jsonUnmarshalerOf(v reflect.Value) json.Unmarshaler {
	if v.Kind() != reflect.Pointer && v.CanAddr() && v.Addr().Type().Implements(jsonUnmarshalerType) {
		return v.Addr().Interface().(json.Unmarshaler)
	}
	if v.Type().Implements(jsonUnmarshalerType) && v.CanInterface() {
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return nil
		}
		return v.Interface().(json.Unmarshaler)
	}
	return nil
}

func textUnmarshalerOf(v reflect.Value) encoding.TextUnmarshaler {
	if v.Kind() != reflect.Pointer && v.CanAddr() && v.Addr().Type().Implements(textUnmarshalerType) {
		return v.Addr().Interface().(encoding.TextUnmarshaler)
	}
	if v.Type().Implements(textUnmarshalerType) && v.CanInterface() {
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return nil
		}
		return v.Interface().(encoding.TextUnmarshaler)
	}
	return nil
}

// jsonNumberOf returns the JSON literal for a numeric value: a json.Number as
// is, or a Go number formatted as one.
func jsonNumberOf(value any) (json.Number, bool) {
	switch n := value.(type) {
	case json.Number:
		return n, true
	case float64:
		return json.Number(strconv.FormatFloat(n, 'g', -1, 64)), true
	case float32:
		return json.Number(strconv.FormatFloat(float64(n), 'g', -1, 32)), true
	case int:
		return json.Number(strconv.FormatInt(int64(n), 10)), true
	case int8:
		return json.Number(strconv.FormatInt(int64(n), 10)), true
	case int16:
		return json.Number(strconv.FormatInt(int64(n), 10)), true
	case int32:
		return json.Number(strconv.FormatInt(int64(n), 10)), true
	case int64:
		return json.Number(strconv.FormatInt(n, 10)), true
	case uint:
		return json.Number(strconv.FormatUint(uint64(n), 10)), true
	case uint8:
		return json.Number(strconv.FormatUint(uint64(n), 10)), true
	case uint16:
		return json.Number(strconv.FormatUint(uint64(n), 10)), true
	case uint32:
		return json.Number(strconv.FormatUint(uint64(n), 10)), true
	case uint64:
		return json.Number(strconv.FormatUint(n, 10)), true
	}
	return "", false
}

// jsonValueToInterface converts a decoded value for an `any` destination the
// way encoding/json would have decoded it there: numbers become float64.
func jsonValueToInterface(value any) (any, error) {
	switch v := value.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return nil, jsonTypeError(value, reflect.TypeFor[float64]())
		}
		return f, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, member := range v {
			conv, err := jsonValueToInterface(member)
			if err != nil {
				return nil, err
			}
			out[k] = conv
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			conv, err := jsonValueToInterface(item)
			if err != nil {
				return nil, err
			}
			out[i] = conv
		}
		return out, nil
	}
	return value, nil
}

func jsonMapKey(name string, kt reflect.Type) (reflect.Value, error) {
	if reflect.PointerTo(kt).Implements(textUnmarshalerType) {
		kv := reflect.New(kt)
		if err := kv.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(name)); err != nil {
			return reflect.Value{}, err
		}
		return kv.Elem(), nil
	}
	switch kt.Kind() {
	case reflect.String:
		return reflect.ValueOf(name).Convert(kt), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(name, 10, 64)
		if err != nil || reflect.Zero(kt).OverflowInt(n) {
			return reflect.Value{}, jsonTypeError(name, kt)
		}
		return reflect.ValueOf(n).Convert(kt), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := strconv.ParseUint(name, 10, 64)
		if err != nil || reflect.Zero(kt).OverflowUint(n) {
			return reflect.Value{}, jsonTypeError(name, kt)
		}
		return reflect.ValueOf(n).Convert(kt), nil
	}
	return reflect.Value{}, fmt.Errorf("%w: map key %s", errUnsupportedJSONDestination, kt)
}

func jsonTypeError(value any, t reflect.Type) error {
	return fmt.Errorf("cannot bind JSON %s into Go value of type %s", jsonValueKind(value), t)
}

func jsonValueKind(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case string:
		return "string"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	}
	if _, ok := jsonNumberOf(value); ok {
		return "number"
	}
	return fmt.Sprintf("value of type %T", value)
}

// jsonField describes one JSON-visible struct field.
type jsonField struct {
	name   string
	index  []int
	tagged bool
	quoted bool
}

var jsonFieldsCache sync.Map // reflect.Type -> map[string]jsonField

func cachedJSONFields(t reflect.Type) map[string]jsonField {
	if f, ok := jsonFieldsCache.Load(t); ok {
		return f.(map[string]jsonField)
	}
	f, _ := jsonFieldsCache.LoadOrStore(t, jsonFields(t))
	return f.(map[string]jsonField)
}

// jsonFields returns the JSON-visible fields of struct type t keyed by their
// JSON member name, following encoding/json's rules: the `json` tag names the
// member or hides the field with "-", untagged exported fields use their Go
// name, embedded structs are promoted breadth-first, and a name claimed at the
// same depth by several fields goes to the single tagged one or to nobody.
func jsonFields(t reflect.Type) map[string]jsonField {
	type queued struct {
		typ   reflect.Type
		index []int
	}
	current := []queued{}
	next := []queued{{typ: t}}
	visited := map[reflect.Type]bool{}
	depth := 0
	byName := map[string][]struct {
		f     jsonField
		depth int
	}{}

	for len(next) > 0 {
		current, next = next, current[:0]
		for _, q := range current {
			if visited[q.typ] {
				continue
			}
			visited[q.typ] = true
			for i := 0; i < q.typ.NumField(); i++ {
				sf := q.typ.Field(i)
				if sf.Anonymous {
					ft := sf.Type
					if ft.Kind() == reflect.Pointer {
						ft = ft.Elem()
					}
					if !sf.IsExported() && ft.Kind() != reflect.Struct {
						continue
					}
				} else if !sf.IsExported() {
					continue
				}
				tag := sf.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, opts, _ := strings.Cut(tag, ",")
				if !isValidJSONTagName(name) {
					name = ""
				}
				index := make([]int, len(q.index)+1)
				copy(index, q.index)
				index[len(q.index)] = i

				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}

				if name == "" && sf.Anonymous && ft.Kind() == reflect.Struct {
					next = append(next, queued{typ: ft, index: index})
					continue
				}
				f := jsonField{
					name:   name,
					index:  index,
					tagged: name != "",
					quoted: hasJSONTagOption(opts, "string") && isQuotableJSONKind(ft),
				}
				if f.name == "" {
					f.name = sf.Name
				}
				byName[f.name] = append(byName[f.name], struct {
					f     jsonField
					depth int
				}{f, depth})
			}
		}
		depth++
	}

	out := make(map[string]jsonField, len(byName))
	for name, group := range byName {
		// The shallowest fields dominate; among them, a single tagged field
		// wins, otherwise a single field wins, otherwise the name is dropped.
		minDepth := group[0].depth
		for _, g := range group[1:] {
			if g.depth < minDepth {
				minDepth = g.depth
			}
		}
		var shallow []jsonField
		for _, g := range group {
			if g.depth == minDepth {
				shallow = append(shallow, g.f)
			}
		}
		if len(shallow) == 1 {
			out[name] = shallow[0]
			continue
		}
		var tagged []jsonField
		for _, f := range shallow {
			if f.tagged {
				tagged = append(tagged, f)
			}
		}
		if len(tagged) == 1 {
			out[name] = tagged[0]
		}
	}
	return out
}

func hasJSONTagOption(opts, name string) bool {
	for opts != "" {
		var opt string
		opt, opts, _ = strings.Cut(opts, ",")
		if opt == name {
			return true
		}
	}
	return false
}

// isQuotableJSONKind reports whether the `,string` option applies to a type,
// as in encoding/json: only booleans, numbers and strings.
func isQuotableJSONKind(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return true
	}
	return false
}

// isValidJSONTagName mirrors encoding/json's rule for tag names: letters,
// digits and a set of punctuation characters; anything else makes the tag
// name ignored in favour of the Go field name.
func isValidJSONTagName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
		case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c > 127:
		default:
			return false
		}
	}
	return true
}

// jsonFieldByIndex walks an index path into v, allocating nil embedded
// pointers along the way, as encoding/json does when it sets a promoted field.
func jsonFieldByIndex(v reflect.Value, index []int) (reflect.Value, error) {
	for i, x := range index {
		if i > 0 {
			if v.Kind() == reflect.Pointer {
				if v.IsNil() {
					if !v.CanSet() {
						return reflect.Value{}, fmt.Errorf("cannot set embedded pointer to unexported struct: %s", v.Type().Elem())
					}
					v.Set(reflect.New(v.Type().Elem()))
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v, nil
}
