package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// NarrowUnionNumericFormats controls whether numeric width formats narrow
// the dynamic type produced when a multi-type union parameter is bound into
// an `any` destination.
//
// When false (the default), the bound value's dynamic type is always one of
// bool, int64, float64, string, or []byte, regardless of the schema's
// `format`: an edit to a spec's format can never change the types a running
// handler's type switch sees. When true, `format: int32` produces int32
// (values outside int32 range fall through to the next union member) and
// `format: float` produces float32, widening the possible dynamic types to
// bool, int32, int64, float32, float64, string, and []byte. `int64` and
// `double` name the defaults either way. Formats never affect concrete
// (non-`any`) destinations, whose Go type was fixed at generation time.
//
// Note one asymmetry with concrete destinations: a concrete int32
// destination rejects an out-of-range value with an overflow error, but an
// `any` destination binds it to the next union member instead — typically
// the string member, verbatim. Enabling narrowing to get int32 typing
// therefore also accepts that silent widening; a handler that needs
// out-of-range values rejected must check for the string case itself.
//
// Like DefaultQueryEncoder, set it once during program initialization; it is
// not safe to mutate concurrently with in-flight requests. The opt-in lives
// here rather than in generated code because the trade-off belongs to
// whoever owns the handler's type switch: enabling it is a promise that the
// application handles the narrowed types.
var NarrowUnionNumericFormats bool

// unionMemberOrder is the order in which union member types are attempted
// when binding a parameter value into an `any` destination: most restrictive
// grammar first, so that the always-succeeding string member cannot shadow
// the others. This is deliberately NOT the schema's declaration order — JSON
// Schema defines the `type` array as an unordered set, so declaration order
// carries no meaning, and any tool that normalizes a spec could otherwise
// silently change binding behavior. The "null" nullability marker and
// non-scalar names ("array", "object") never appear here, so they are
// structurally skipped during the walk regardless of what the generator
// emitted in Types.
var unionMemberOrder = [4]string{"boolean", "integer", "number", "string"}

// bindStringToUnionMember binds src against the members of an OpenAPI 3.1
// multi-type union (opts.Types), returning the value of the first member
// that parses. Members are tried in unionMemberOrder, restricted to the
// members actually present in opts.Types.
//
// Numeric detection uses the JSON number production (RFC 8259 section 6),
// not strconv leniency: "007", "+1" and " 1" are not JSON numbers, so they
// fall through to the string member rather than being silently
// reinterpreted.
//
// The dynamic type of the returned value is one of exactly bool, int64,
// float64, string, or — with Format "byte" — []byte. Width formats (int32,
// int64, float, double) are annotation-only by default and do not narrow
// the produced type: honoring them would mean an edit to a spec's `format`
// silently changes the dynamic type a running handler's type switch sees,
// with no compile error. Applications that want width narrowing opt in via
// the NarrowUnionNumericFormats package variable. "byte" is always
// load-bearing because it changes the wire decoding (base64) rather than a
// width; other annotation-only formats (date-time, uuid, ...) are ignored —
// per OpenAPI 3.1 semantics `format` is an annotation and must not reject a
// value, so parse failure cannot discriminate members. A format whose host
// type is not present in opts.Types is inert.
//
// Non-scalar member names ("array", "object"), the "null" nullability marker
// and unknown names are skipped: styled serialization of those into `any`
// has no defined meaning. If no member parses, an error naming the union's
// bindable members is returned.
func bindStringToUnionMember(src string, opts BindStringToObjectOptions) (any, error) {
	for _, name := range unionMemberOrder {
		if !unionHasMember(opts.Types, name) {
			continue
		}
		switch name {
		case "boolean":
			// JSON grammar: exactly the lowercase literals, unlike
			// strconv.ParseBool which also accepts "1", "t", "TRUE", etc.
			if src == "true" {
				return true, nil
			}
			if src == "false" {
				return false, nil
			}
		case "integer":
			if isJSONInteger(src) {
				if NarrowUnionNumericFormats && opts.Format == "int32" {
					if val, err := strconv.ParseInt(src, 10, 32); err == nil {
						return int32(val), nil
					}
				} else if val, err := strconv.ParseInt(src, 10, 64); err == nil {
					return val, nil
				}
				// Overflow of the (possibly narrowed) width: not
				// representable as this member, fall through to the next
				// one (number takes it as a float, string takes it
				// verbatim).
			}
		case "number":
			if isJSONNumber(src) {
				if NarrowUnionNumericFormats && opts.Format == "float" {
					if val, err := strconv.ParseFloat(src, 32); err == nil {
						return float32(val), nil
					}
				} else if val, err := strconv.ParseFloat(src, 64); err == nil {
					return val, nil
				}
				// Out of range for the (possibly narrowed) width: fall
				// through.
			}
		case "string":
			if opts.Format == "byte" {
				// Consistent with the concrete []byte destination: a
				// declared base64 wire encoding that doesn't decode is an
				// error, not a silent fallback to the raw string.
				// base64Decode's error already names the offending value.
				return base64Decode(src)
			}
			return src, nil
		}
	}

	// Name only the bindable members in the error: the generator is expected
	// to strip the "null" nullability marker before emitting Types, but the
	// runtime and generator version independently, so don't rely on it.
	members := make([]string, 0, len(opts.Types))
	for _, name := range opts.Types {
		if name != "null" {
			members = append(members, name)
		}
	}
	if len(members) == 0 {
		// Degenerate input (e.g. Types: ["null"]): say so instead of
		// printing "type union []", which would read as a runtime bug.
		return nil, fmt.Errorf("value '%s' can not bind: type union has no bindable members (declared %v)", src, opts.Types)
	}
	return nil, fmt.Errorf("value '%s' does not match any member of type union %v", src, members)
}

// unionHasMember reports whether name appears in types. A linear scan: the
// list has at most a handful of entries and this runs per parameter per
// request, so avoiding a map allocation matters more than big-O.
func unionHasMember(types []string, name string) bool {
	for _, t := range types {
		if t == name {
			return true
		}
	}
	return false
}

// isJSONNumber reports whether s is a number under JSON grammar (RFC 8259):
// an optional leading '-', an integer part with no leading zeros, and
// optional fraction and exponent parts. No '+' sign, no whitespace, no hex.
func isJSONNumber(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	// Integer part: "0", or a nonzero digit followed by digits.
	if i >= len(s) {
		return false
	}
	switch {
	case s[i] == '0':
		i++
	case s[i] >= '1' && s[i] <= '9':
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	default:
		return false
	}
	// Fraction part.
	if i < len(s) && s[i] == '.' {
		i++
		if i >= len(s) || !isDigit(s[i]) {
			return false
		}
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	// Exponent part.
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i >= len(s) || !isDigit(s[i]) {
			return false
		}
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	return i == len(s)
}

// isJSONInteger reports whether s is an integer token under JSON grammar: a
// JSON number with no fraction or exponent part. This deliberately rejects
// strconv leniencies like "007" or "+1", which would silently change the
// value ("007" binds as the string "007", not the integer 7).
func isJSONInteger(s string) bool {
	return isJSONNumber(s) && !strings.ContainsAny(s, ".eE")
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
