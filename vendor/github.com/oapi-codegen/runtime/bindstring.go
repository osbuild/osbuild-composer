// Copyright 2019 DeepMap, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package runtime

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/oapi-codegen/runtime/types"
)

// BindStringToObject takes a string, and attempts to assign it to the destination
// interface via whatever type conversion is necessary. We have to do this
// via reflection instead of a much simpler type switch so that we can handle
// type aliases. This function was the easy way out, the better way, since we
// know the destination type each place that we use this, is to generate code
// to read each specific type.
func BindStringToObject(src string, dst interface{}) error {
	return BindStringToObjectWithOptions(src, dst, BindStringToObjectOptions{})
}

// BindStringToObjectOptions defines optional arguments for BindStringToObjectWithOptions.
type BindStringToObjectOptions struct {
	// Type is the OpenAPI type of the parameter (e.g. "string", "integer").
	Type string
	// Format is the OpenAPI format of the parameter (e.g. "byte", "date-time").
	// When set to "byte" and the destination is []byte, the source string is
	// base64-decoded rather than treated as a generic slice.
	Format string
	// Types is the OpenAPI 3.1 multi-type union member list of the parameter
	// (e.g. ["string", "integer"]). A "null" entry — the 3.1 nullability
	// marker, not a union member — is ignored, whether or not the generator
	// already stripped it. (Type, which the runtime does not currently read,
	// carries no meaning when Types is set.)
	//
	// Types is only consulted when the destination is an empty interface
	// (`any`): the source string is bound to the first member that parses,
	// trying boolean, integer, number, then string (most restrictive grammar
	// first — the always-succeeding string member would otherwise shadow the
	// rest). Numeric detection uses the JSON number production (RFC 8259
	// section 6), so tokens like "007" and "+1" bind as strings. The bound
	// value's dynamic type is one of exactly bool, int64, float64, string,
	// or, with Format "byte", []byte; width formats (int32, float, ...) are
	// annotation-only unless the application opts into narrowing via the
	// NarrowUnionNumericFormats package variable.
	//
	// Concrete destinations ignore this field and keep the reflection-driven
	// behavior. Array element binding does not yet support unions, and
	// deepObject-style binding does not consult this field (its JSON decode
	// path produces float64 for all numbers).
	Types []string
}

// BindStringToObjectWithOptions takes a string, and attempts to assign it to the destination
// interface via whatever type conversion is necessary, with additional options.
func BindStringToObjectWithOptions(src string, dst interface{}, opts BindStringToObjectOptions) error {
	var err error

	// Check if the destination implements Binder interface before any reflection
	if binder, ok := dst.(Binder); ok {
		return binder.Bind(src)
	}

	v := reflect.ValueOf(dst)
	t := reflect.TypeOf(dst)

	// We need to dereference pointers
	if t.Kind() == reflect.Pointer {
		v = reflect.Indirect(v)
		t = v.Type()
	}

	// For some optional args
	if t.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(t.Elem()))
		}

		v = reflect.Indirect(v)
		t = v.Type()
	}

	// The resulting type must be settable. reflect will catch issues like
	// passing the destination by value.
	if !v.CanSet() {
		return errors.New("destination is not settable")
	}

	switch t.Kind() {
	case reflect.Slice:
		if opts.Format == "byte" && isByteSlice(t) {
			decoded, decErr := base64Decode(src)
			if decErr != nil {
				return fmt.Errorf("error binding string parameter: %w", decErr)
			}
			v.SetBytes(decoded)
			return nil
		}
		// Non-binary slices have no string representation to parse, so they
		// get the same unhandled-type error as the default case below. This
		// can not be a fallthrough: the next case is the integer one, and a
		// source string that parses as an integer would reach v.OverflowInt
		// on a slice value, which panics.
		err = fmt.Errorf("can not bind to destination of type: %s", t.Kind())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var val int64
		val, err = strconv.ParseInt(src, 10, 64)
		if err == nil {
			if v.OverflowInt(val) {
				err = fmt.Errorf("value '%s' overflows destination of type: %s", src, t.Kind())
			}
			if err == nil {
				v.SetInt(val)
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var val uint64
		val, err = strconv.ParseUint(src, 10, 64)
		if err == nil {
			if v.OverflowUint(val) {
				err = fmt.Errorf("value '%s' overflows destination of type: %s", src, t.Kind())
			}
			v.SetUint(val)
		}
	case reflect.String:
		v.SetString(src)
		err = nil
	case reflect.Float64, reflect.Float32:
		var val float64
		val, err = strconv.ParseFloat(src, 64)
		if err == nil {
			if v.OverflowFloat(val) {
				err = fmt.Errorf("value '%s' overflows destination of type: %s", src, t.Kind())
			}
			v.SetFloat(val)
		}
	case reflect.Bool:
		var val bool
		val, err = strconv.ParseBool(src)
		if err == nil {
			v.SetBool(val)
		}
	case reflect.Array:
		if tu, ok := dst.(encoding.TextUnmarshaler); ok {
			if err := tu.UnmarshalText([]byte(src)); err != nil {
				return fmt.Errorf("error unmarshaling '%s' text as %T: %w", src, dst, err)
			}

			return nil
		}
		fallthrough
	case reflect.Struct:
		if t.ConvertibleTo(reflect.TypeOf(time.Time{})) {
			// Don't fail on empty string.
			if src == "" {
				return nil
			}
			// Time is a special case of a struct that we handle
			parsedTime, err := time.Parse(time.RFC3339Nano, src)
			if err != nil {
				parsedTime, err = time.Parse(types.DateFormat, src)
				if err != nil {
					return fmt.Errorf("error parsing '%s' as RFC3339 or 2006-01-02 time: %w", src, err)
				}
			}
			// So, assigning this gets a little fun. We have a value to the
			// dereference destination. We can't do a conversion to
			// time.Time because the result isn't assignable, so we need to
			// convert pointers.
			if t != reflect.TypeOf(time.Time{}) {
				vPtr := v.Addr()
				vtPtr := vPtr.Convert(reflect.TypeOf(&time.Time{}))
				v = reflect.Indirect(vtPtr)
			}
			v.Set(reflect.ValueOf(parsedTime))
			return nil
		}

		if t.ConvertibleTo(reflect.TypeOf(types.Date{})) {
			// Don't fail on empty string.
			if src == "" {
				return nil
			}
			parsedTime, err := time.Parse(types.DateFormat, src)
			if err != nil {
				return fmt.Errorf("error parsing '%s' as date: %w", src, err)
			}
			parsedDate := types.Date{Time: parsedTime}

			// We have to do the same dance here to assign, just like with times
			// above.
			if t != reflect.TypeOf(types.Date{}) {
				vPtr := v.Addr()
				vtPtr := vPtr.Convert(reflect.TypeOf(&types.Date{}))
				v = reflect.Indirect(vtPtr)
			}
			v.Set(reflect.ValueOf(parsedDate))
			return nil
		}

		// We fall through to the error case below if we haven't handled the
		// destination type above.
		fallthrough
	case reflect.Interface:
		// An interface destination normally can't be bound: there is no
		// type information to parse with, so it falls to the error below.
		// The exception is an empty interface (`any`) destination for a
		// declared OpenAPI 3.1 multi-type union — opts.Types names the
		// member types, and the value binds to the first member that
		// parses. See bindStringToUnionMember for the exact semantics.
		if t.Kind() == reflect.Interface && t.NumMethod() == 0 && len(opts.Types) > 0 {
			bound, bindErr := bindStringToUnionMember(src, opts)
			if bindErr != nil {
				return fmt.Errorf("error binding string parameter: %w", bindErr)
			}
			v.Set(reflect.ValueOf(bound))
			return nil
		}
		fallthrough
	case reflect.Map:
		// A bool-keyed map (such as nullable.Nullable[T], which is
		// map[bool]T) is treated as a nullable wrapper: bind src into a
		// fresh value of the inner type and store it under map[true].
		if t.Kind() == reflect.Map && t.Key().Kind() == reflect.Bool {
			elemPtr := reflect.New(t.Elem())
			if bindErr := BindStringToObjectWithOptions(src, elemPtr.Interface(), opts); bindErr != nil {
				return bindErr
			}
			newMap := reflect.MakeMap(t)
			newMap.SetMapIndex(reflect.ValueOf(true), elemPtr.Elem())
			v.Set(newMap)
			return nil
		}
		fallthrough
	default:
		// We've got a bunch of types unimplemented, don't fail silently.
		err = fmt.Errorf("can not bind to destination of type: %s", t.Kind())
	}
	if err != nil {
		return fmt.Errorf("error binding string parameter: %w", err)
	}
	return nil
}
