package runner

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
)

// 1. Walk the struct
// 2. If the field is a string, check if it has the template tag
// 3. Extract the CEL expressions from the string
// 4. Track the CEL expressions and their locations

const (
	celExprStart = "${"
	celExprEnd   = "}"
)

type TemplateExpression struct {
	Expression string
	References []string
	Program    cel.Program
}

type TemplateField struct {
	Path                 string
	Expressions          []TemplateExpression
	StandaloneExpression bool
}

func ParseTemplate(in map[string]any) ([]TemplateField, error) {
	fields := []TemplateField{}
	return fields, nil
}

func parseTemplate(in any, path string) ([]TemplateField, error) {
	switch v := in.(type) {
	case map[string]any:
		return parseObject(v, path)
	case []any:
		return parseArray(v, path)
	case string:
		return parseString(v, path)
	default:
		return nil, nil
	}
}

func parseObject(in map[string]any, path string) ([]TemplateField, error) {
	fields := []TemplateField{}
	for key, value := range in {
		childFields, err := parseTemplate(value, buildPath(path, key))
		if err != nil {
			return nil, err
		}
		fields = append(fields, childFields...)
	}
	return fields, nil
}

func parseArray(in []any, path string) ([]TemplateField, error) {
	fields := []TemplateField{}
	for i, value := range in {
		childFields, err := parseTemplate(value, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		fields = append(fields, childFields...)
	}
	return fields, nil
}

func parseString(in string, path string) ([]TemplateField, error) {
	fields := []TemplateField{}
	expressions, err := extractExpressions(in)
	if err != nil {
		return nil, err
	}

	fields = append(fields, TemplateField{
		Path:                 path,
		Expressions:          expressions,
		StandaloneExpression: len(expressions) == 1 && in == fmt.Sprintf("%s%s%s", celExprStart, expressions[0], celExprEnd),
	})

	return fields, nil
}

func buildPath(path string, key string) string {
	if key == "" || strings.Contains(key, ".") {
		return fmt.Sprintf("%s[%q]", path, key)
	}

	if path == "" {
		return fmt.Sprintf("[%q]", key)
	}

	return fmt.Sprintf("%s.%s", path, key)
}

func extractExpressions(in string) ([]TemplateExpression, error) {
	expressions := []TemplateExpression{}

	start := 0
	// Iterate over the string and find all expressions
	for start < len(in) {
		// Find the start of the next expression. If none is found, break
		startIdx := strings.Index(in[start:], celExprStart)
		if startIdx == -1 {
			break
		}
		// Adjust the start index to the actual position in the string
		startIdx += start

		// We need to find the matching end bracket, being careful about
		// nested expressions, dictionary building expressions, and string literals
		bracketCount := 1
		endIdx := startIdx + len(celExprStart)
		inStringLiteral := false
		escapeNext := false

		for endIdx < len(in) {
			c := in[endIdx]

			// Handle escape sequences inside string literals
			if escapeNext {
				escapeNext = false
				endIdx++
				continue
			}

			// Check for escape character inside string literals
			if inStringLiteral && c == '\\' {
				escapeNext = true
				endIdx++
				continue
			}

			// Handle string literal boundaries
			if c == '"' {
				inStringLiteral = !inStringLiteral
			} else if !inStringLiteral {
				// Only count braces when not inside a string literal
				if c == '{' {
					bracketCount++
				} else if c == '}' {
					bracketCount--
					if bracketCount == 0 {
						break
					}
				} else if endIdx+1 < len(in) && in[endIdx:endIdx+len(celExprStart)] == celExprStart {
					// Allow nested expressions, but only if they are escaped with quotes
					if in[endIdx-1] != '"' {
						return nil, fmt.Errorf("nested expression not allowed")
					}
				}
			}
			endIdx++
		}

		if bracketCount != 0 {
			// Incomplete expression, move to next character and continue
			start++
			continue
		}

		// The expression is the substring between the start and end indices
		// of '${' and the matching '}'
		expr := in[startIdx+len(celExprStart) : endIdx]
		expressions = append(expressions, TemplateExpression{
			Expression: expr,
			References: []string{},
			Program:    nil,
		})
		start = endIdx + 1
	}
	return expressions, nil
}

// // ExpandTemplates walks the struct (or slice of structs) pointed to by in and
// // updates template fields in place. It explores all nested structs, *struct,
// // map[string]string, []string, and []struct/[]*struct recursively. String, *string,
// // and []string fields are gated by the `template` struct tag: `template` or `template:""`
// // means expand via Expand; `template:"-"` means skip. All other explorable types
// // are traversed without requiring the tag.
// //
// // Types: string, *string, []string (template tag required; nil left as-is),
// // map[string]string (always ExpandMap; nil left as-is), struct (recurse),
// // *struct (recurse; nil skipped), []struct, []*struct (recurse). Unexported
// // fields are skipped.
// func ExpandTemplates[T any](in *T, variables map[string]string) error {
// 	if in == nil {
// 		return nil
// 	}
// 	v := reflect.ValueOf(in).Elem()
// 	switch v.Kind() {
// 	case reflect.Struct:
// 		return expandStructInPlace(v, variables)
// 	case reflect.Slice:
// 		return expandSliceInPlace(v, variables)
// 	default:
// 		return fmt.Errorf("ExpandTemplates expects *struct or *[]struct; got *%s", v.Type())
// 	}
// }

// func expandSliceInPlace(v reflect.Value, variables map[string]string) error {
// 	if v.IsNil() {
// 		return nil
// 	}
// 	elemTyp := v.Type().Elem()
// 	switch {
// 	case elemTyp.Kind() == reflect.String:
// 		for i := 0; i < v.Len(); i++ {
// 			el := v.Index(i)
// 			expanded, err := Expand(el.String(), variables)
// 			if err != nil {
// 				return err
// 			}
// 			el.SetString(expanded)
// 		}
// 		return nil
// 	case elemTyp.Kind() == reflect.Struct:
// 		for i := 0; i < v.Len(); i++ {
// 			if err := expandStructInPlace(v.Index(i), variables); err != nil {
// 				return err
// 			}
// 		}
// 		return nil
// 	case elemTyp.Kind() == reflect.Ptr && elemTyp.Elem().Kind() == reflect.Struct:
// 		for i := 0; i < v.Len(); i++ {
// 			el := v.Index(i)
// 			if el.IsNil() {
// 				continue
// 			}
// 			if err := expandStructInPlace(el.Elem(), variables); err != nil {
// 				return err
// 			}
// 		}
// 		return nil
// 	default:
// 		return nil
// 	}
// }

// func expandStructInPlace(v reflect.Value, variables map[string]string) error {
// 	if v.Kind() != reflect.Struct {
// 		return fmt.Errorf("expandStructInPlace expects struct; got %s", v.Kind())
// 	}
// 	typ := v.Type()
// 	for i := 0; i < typ.NumField(); i++ {
// 		sf := typ.Field(i)
// 		if !sf.IsExported() {
// 			continue
// 		}
// 		field := v.Field(i)
// 		tag, hasTemplate := sf.Tag.Lookup("template")

// 		switch field.Kind() {
// 		case reflect.String:
// 			if !hasTemplate || tag == "-" {
// 				continue
// 			}
// 			expanded, err := Expand(field.String(), variables)
// 			if err != nil {
// 				return err
// 			}
// 			field.SetString(expanded)

// 		case reflect.Ptr:
// 			if field.IsNil() {
// 				continue
// 			}
// 			elem := field.Elem()
// 			switch elem.Kind() {
// 			case reflect.String:
// 				if !hasTemplate || tag == "-" {
// 					continue
// 				}
// 				expanded, err := Expand(elem.String(), variables)
// 				if err != nil {
// 					return err
// 				}
// 				newPtr := reflect.New(elem.Type())
// 				newPtr.Elem().SetString(expanded)
// 				field.Set(newPtr)
// 			case reflect.Struct:
// 				if err := expandStructInPlace(elem, variables); err != nil {
// 					return err
// 				}
// 			default:
// 				continue
// 			}

// 		case reflect.Map:
// 			if field.Type().Key().Kind() != reflect.String || field.Type().Elem().Kind() != reflect.String {
// 				continue
// 			}
// 			if field.IsNil() {
// 				continue
// 			}
// 			expanded, err := ExpandMap(field.Interface().(map[string]string), variables)
// 			if err != nil {
// 				return err
// 			}
// 			field.Set(reflect.ValueOf(expanded))

// 		case reflect.Struct:
// 			if err := expandStructInPlace(field, variables); err != nil {
// 				return err
// 			}

// 		case reflect.Slice:
// 			if field.Type().Elem().Kind() == reflect.String && (!hasTemplate || tag == "-") {
// 				continue
// 			}
// 			if err := expandSliceInPlace(field, variables); err != nil {
// 				return err
// 			}

// 		default:
// 			continue
// 		}
// 	}
// 	return nil
// }

// // Expand replaces ${VAR} references in the input string using the provided variables map.
// // Returns an error if any referenced variable is not in the variables map.
// func Expand(value string, variables map[string]string) (string, error) {
// 	var errs error

// 	result := os.Expand(value, func(key string) string {
// 		if val, ok := variables[key]; ok {
// 			return val
// 		}
// 		errs = errors.Join(errs, fmt.Errorf("environment variable %q is not in the allowed list", key))
// 		return ""
// 	})

// 	if errs != nil {
// 		return "", errs
// 	}

// 	return result, nil
// }

// // ExpandMap expands all values in a map[string]string.
// // Returns an error if any value fails to expand.
// func ExpandMap(values map[string]string, variables map[string]string) (map[string]string, error) {
// 	if values == nil {
// 		return nil, nil
// 	}

// 	result := make(map[string]string, len(values))
// 	var errs error

// 	for k, v := range values {
// 		expanded, err := Expand(v, variables)
// 		if err != nil {
// 			errs = errors.Join(errs, err)
// 			continue
// 		}
// 		result[k] = expanded
// 	}

// 	if errs != nil {
// 		return nil, errs
// 	}

// 	return result, nil
// }
