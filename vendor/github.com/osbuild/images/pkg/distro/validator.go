package distro

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
)

type ImageTypeValidator interface {
	// A list of customization options that this image requires.
	RequiredBlueprintOptions() []string

	// A list of customization options that this image supports.
	SupportedBlueprintOptions() []string
}

type validationError struct {
	// Reverse path to the customization that caused the error.
	revPath []string
	message string
}

func (e validationError) Error() string {
	path := e.revPath
	slices.Reverse(path)
	return fmt.Sprintf("%s: %s", strings.Join(path, "."), e.message)
}

func validateSupportedConfig(supported []string, conf reflect.Value) *validationError {

	// Construct two maps:
	//  - subMap represents the keys on the current level of the recursion that
	//  have sub-keys in the list of supported customizations:
	//  - supportedMap represents the keys on the current level that are fully
	//  supported.
	//
	// For example, for the following customizations
	//   customizations.kernel.name
	//   customizations.locale
	//
	// subMap will be
	//   {"customizations": ["kernel.name", "locale"]}
	//
	// When the function is then recursively called with just the "locale"
	// element, supportedMap will be
	//   {"locale": true}

	supportedMap := make(map[string]bool)
	subMap := make(map[string][]string)

	for _, key := range supported {
		if strings.Contains(key, ".") {
			// nested key: add top level component as key in subMap and the
			// rest as the value.
			parts := strings.SplitN(key, ".", 2)
			subList := subMap[parts[0]]
			subList = append(subList, parts[1])
			subMap[parts[0]] = subList
		} else {
			// leaf node in supported list: will be checked for non-zero value
			supportedMap[key] = true
		}
	}

	confT := conf.Type()
	for fieldIdx := 0; fieldIdx < confT.NumField(); fieldIdx++ {
		field := confT.Field(fieldIdx)
		if field.Anonymous {
			// embedded struct: flatten with the parent
			if err := validateSupportedConfig(supported, conf.Field(fieldIdx)); err != nil {
				return err
			}
			continue
		}

		tag := jsonTagFor(field)
		subList, listed := subMap[tag]
		if !listed {
			// not listed: check if it's non-zero
			empty := conf.Field(fieldIdx).IsZero()
			if !empty && !supportedMap[tag] {
				return &validationError{message: "not supported", revPath: []string{tag}}
			}
			continue
		}

		subStruct := conf.Field(fieldIdx)
		if subStruct.IsZero() {
			// nothing to validate: continue
			continue
		}
		if subStruct.Kind() == reflect.Ptr {
			// dereference pointer before validating
			subStruct = subStruct.Elem()
		}

		switch subStruct.Kind() {
		case reflect.Slice:
			// iterate over slice and validate each element as a substructure
			for sliceIdx := 0; sliceIdx < subStruct.Len(); sliceIdx++ {
				if err := validateSupportedConfig(subList, subStruct.Index(sliceIdx)); err != nil {
					err.revPath = append(err.revPath, fmt.Sprintf("%s[%d]", tag, sliceIdx))
					return err
				}
			}
		case reflect.Struct:
			// single element
			if err := validateSupportedConfig(subList, subStruct); err != nil {
				err.revPath = append(err.revPath, tag)
				return err
			}
		case reflect.Int, reflect.Bool, reflect.String:
			// this can happen if the supported list contains an invalid
			// string, where a non-container type field is followed by a
			// period, for example, "a.b" where a is an integer
			return &validationError{message: fmt.Sprintf("internal error: supported list specifies child element of non-container type %v: %v", subStruct.Kind(), subStruct), revPath: []string{tag}}
		default:
			// this can happen if the config uses a container type that's
			// not a struct or an array (e.g. a map).
			return &validationError{message: fmt.Sprintf("internal error: unexpected field type: %v (%v)", subStruct.Kind(), subStruct), revPath: []string{tag}}
		}
	}

	return nil
}

func jsonTagFor(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	return strings.Split(tag, ",")[0]
}

func fieldByTag(p reflect.Value, tag string) (reflect.Value, error) {
	for idx := 0; idx < p.Type().NumField(); idx++ {
		c := p.Type().Field(idx)
		if c.Anonymous {
			// embedded struct: flatten with the parent
			value, err := fieldByTag(p.Field(idx), tag)
			if err != nil {
				// tag not found in embedded struct, continue to check the rest
				continue
			}
			return value, nil
		}
		if jsonTagFor(c) == tag {
			return p.Field(idx), nil
		}
	}

	return reflect.Value{}, fmt.Errorf("%s does not have a field with JSON tag %q", p.Type().Name(), tag)
}

func validateRequiredConfig(required []string, conf reflect.Value) *validationError {
	// create two maps from the required list:
	//
	// 1. requiredMap contains the keys that must exist at this level as
	//    non-zero values. A key in this map can be the name of a substructure
	//    of the blueprint, like "Kernel", in which case that indicates that
	//    the "Kernel" section should be non-zero, regardless of which subparts
	//    of that structure are required or supported.
	//    This differs from the supportedMap in validateSupportedConfig() in
	//    that the requiredMap also lists keys that have required subparts,
	//    whether they are wholly required or not.
	//
	// 2. subMap contains the keys that have sub-parts that are required. Each
	//    substructure will have to be checked recursively until we reach
	//    required leaf nodes.

	requiredMap := make(map[string]bool)
	subMap := make(map[string][]string)
	for _, key := range required {
		if strings.Contains(key, ".") {
			// nested key: add to sub
			parts := strings.SplitN(key, ".", 2)
			subList := subMap[parts[0]]
			subList = append(subList, parts[1])
			subMap[parts[0]] = subList

			// if any subkey is required, then the top level one is as well
			requiredMap[parts[0]] = true
		} else {
			requiredMap[key] = true
		}
	}

	for key := range requiredMap {
		// requiredMap contains keys that are required at this level, whether
		// they have subkeys or not.
		// Their values should be non-zero but only for certain types:
		//   Struct, Pointer, Slice, and String
		// The Zero value for other types could be a valid value, so we
		// shouldn't assume that a zero value is the same as a missing one.
		value, err := fieldByTag(conf, key)
		if err != nil {
			return &validationError{message: err.Error(), revPath: []string{key}}
		}
		switch value.Kind() {
		case reflect.Ptr, reflect.Struct, reflect.String, reflect.Slice:
			// Required should only be used for Pointer, String, and Slice types.
			// For other types, the zero value can be valid and not indicate a
			// missing value.
			if value.IsZero() {
				return &validationError{message: "required", revPath: []string{key}}
			}
		default:
			return &validationError{message: fmt.Sprintf("field of type %v cannot be marked required", value.Kind()), revPath: []string{key}}
		}
	}

	for key := range subMap {
		// subMap contains keys that should contain specific subkeys.
		// If the key's value is Zero, that's an error, but that should be
		// caught by the requiredMap checks above.
		// If it's a Struct, descend into it.
		// If it's s Slice, descend into each element.
		value, err := fieldByTag(conf, key)
		if err != nil {
			return &validationError{message: err.Error(), revPath: []string{key}}
		}
		if value.Kind() == reflect.Ptr {
			// Dereference pointer before validating.
			// We don't need to worry about Zero values because of the previous
			// check in iteration through requiredMap above.
			value = value.Elem()
		}
		switch value.Kind() {
		case reflect.Struct:
			// Descend into map
			if err := validateRequiredConfig(subMap[key], value); err != nil {
				err.revPath = append(err.revPath, key)
				return err
			}
		case reflect.Slice:
			// iterate over slice and validate each element
			for idx := 0; idx < value.Len(); idx++ {
				if err := validateRequiredConfig(subMap[key], value.Index(idx)); err != nil {
					err.revPath = append(err.revPath, fmt.Sprintf("%s[%d]", key, idx))
					return err
				}
			}
		case reflect.String:
			// this can happen if the required list contains an invalid
			// string, where a non-container type field is followed by a
			// period, for example, "a.b" where a is a string
			return &validationError{message: fmt.Sprintf("internal error: required list specifies child element of non-container type %v: %v", value.Kind(), value), revPath: []string{key}}
		default:
			// this should never happen, because we check above that only
			// struct, string, and slice types can be required (and ptr types
			// are dereferenced before the switch)
			return &validationError{message: fmt.Sprintf("internal error: unexpected field type: %v (%v)", value.Kind(), value), revPath: []string{key}}
		}
	}
	return nil
}

func ValidateConfig(t ImageTypeValidator, bp blueprint.Blueprint) error {
	bpv := reflect.ValueOf(bp)
	if err := validateSupportedConfig(t.SupportedBlueprintOptions(), bpv); err != nil {
		return err
	}

	// note that validateRequiredConfig() returns a *validationError not a
	// normal "error", hence the special handling below
	if err := validateRequiredConfig(t.RequiredBlueprintOptions(), bpv); err != nil {
		return err
	}

	// explicitly return nil when there is no error, otherwise the error type
	// will be validationError instead of nil
	// https://go.dev/doc/faq#nil_error
	return nil
}
