package runner

import (
	"errors"
	"fmt"
	"os"
	"reflect"
)

// ExpandTemplates walks the struct (or slice of structs) pointed to by in and
// updates template fields in place. It explores all nested structs, *struct,
// map[string]string, []string, and []struct/[]*struct recursively. String, *string,
// and []string fields are gated by the `template` struct tag: `template` or `template:""`
// means expand via Expand; `template:"-"` means skip. All other explorable types
// are traversed without requiring the tag.
//
// Types: string, *string, []string (template tag required; nil left as-is),
// map[string]string (always ExpandMap; nil left as-is), struct (recurse),
// *struct (recurse; nil skipped), []struct, []*struct (recurse). Unexported
// fields are skipped.
func ExpandTemplates[T any](in *T, variables map[string]string) error {
	if in == nil {
		return nil
	}
	v := reflect.ValueOf(in).Elem()
	switch v.Kind() {
	case reflect.Struct:
		return expandStructInPlace(v, variables)
	case reflect.Slice:
		return expandSliceInPlace(v, variables)
	default:
		return fmt.Errorf("ExpandTemplates expects *struct or *[]struct; got *%s", v.Type())
	}
}

func expandSliceInPlace(v reflect.Value, variables map[string]string) error {
	if v.IsNil() {
		return nil
	}
	elemTyp := v.Type().Elem()
	switch {
	case elemTyp.Kind() == reflect.String:
		for i := 0; i < v.Len(); i++ {
			el := v.Index(i)
			expanded, err := Expand(el.String(), variables)
			if err != nil {
				return err
			}
			el.SetString(expanded)
		}
		return nil
	case elemTyp.Kind() == reflect.Struct:
		for i := 0; i < v.Len(); i++ {
			if err := expandStructInPlace(v.Index(i), variables); err != nil {
				return err
			}
		}
		return nil
	case elemTyp.Kind() == reflect.Ptr && elemTyp.Elem().Kind() == reflect.Struct:
		for i := 0; i < v.Len(); i++ {
			el := v.Index(i)
			if el.IsNil() {
				continue
			}
			if err := expandStructInPlace(el.Elem(), variables); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func expandStructInPlace(v reflect.Value, variables map[string]string) error {
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expandStructInPlace expects struct; got %s", v.Kind())
	}
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		sf := typ.Field(i)
		if !sf.IsExported() {
			continue
		}
		field := v.Field(i)
		tag, hasTemplate := sf.Tag.Lookup("template")

		switch field.Kind() {
		case reflect.String:
			if !hasTemplate || tag == "-" {
				continue
			}
			expanded, err := Expand(field.String(), variables)
			if err != nil {
				return err
			}
			field.SetString(expanded)

		case reflect.Ptr:
			if field.IsNil() {
				continue
			}
			elem := field.Elem()
			switch elem.Kind() {
			case reflect.String:
				if !hasTemplate || tag == "-" {
					continue
				}
				expanded, err := Expand(elem.String(), variables)
				if err != nil {
					return err
				}
				newPtr := reflect.New(elem.Type())
				newPtr.Elem().SetString(expanded)
				field.Set(newPtr)
			case reflect.Struct:
				if err := expandStructInPlace(elem, variables); err != nil {
					return err
				}
			default:
				continue
			}

		case reflect.Map:
			if field.Type().Key().Kind() != reflect.String || field.Type().Elem().Kind() != reflect.String {
				continue
			}
			if field.IsNil() {
				continue
			}
			expanded, err := ExpandMap(field.Interface().(map[string]string), variables)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(expanded))

		case reflect.Struct:
			if err := expandStructInPlace(field, variables); err != nil {
				return err
			}

		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String && (!hasTemplate || tag == "-") {
				continue
			}
			if err := expandSliceInPlace(field, variables); err != nil {
				return err
			}

		default:
			continue
		}
	}
	return nil
}

// Expand replaces ${VAR} references in the input string using the provided variables map.
// Returns an error if any referenced variable is not in the variables map.
func Expand(value string, variables map[string]string) (string, error) {
	var errs error

	result := os.Expand(value, func(key string) string {
		if val, ok := variables[key]; ok {
			return val
		}
		errs = errors.Join(errs, fmt.Errorf("environment variable %q is not in the allowed list", key))
		return ""
	})

	if errs != nil {
		return "", errs
	}

	return result, nil
}

// ExpandMap expands all values in a map[string]string.
// Returns an error if any value fails to expand.
func ExpandMap(values map[string]string, variables map[string]string) (map[string]string, error) {
	if values == nil {
		return nil, nil
	}

	result := make(map[string]string, len(values))
	var errs error

	for k, v := range values {
		expanded, err := Expand(v, variables)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		result[k] = expanded
	}

	if errs != nil {
		return nil, errs
	}

	return result, nil
}
