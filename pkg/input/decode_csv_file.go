package input

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// DecodeCSVFile reads a CSV file from the specified path and decodes its contents
// into a slice of structs of type T. The struct fields must be tagged with `csv`
// tags corresponding to the CSV column headers.
//
// The function validates that all required CSV columns defined by the struct tags
// exist in the file. Extra CSV columns are allowed and ignored.
//
// Supported field types include string, int (and int32/int64), and pointers to these types.
//
// Returns an error if the file is empty, the CSV is malformed, required columns are missing,
// unsupported field types are encountered, or conversion errors occur.
//
// Example usage:
//
//	type User struct {
//	    ID    string  `csv:"id"`
//	    Name  string  `csv:"name"`
//	    Score *int    `csv:"score"`
//	}
//
//	users, err := DecodeCSVFile[User]("users.csv")
//	if err != nil {
//	    // handle error
//	}
func DecodeCSVFile[T any](filePath string) ([]T, error) {
	records, err := ReadCSVLines(filePath)
	if err != nil {
		return nil, fmt.Errorf("error loading data file: %w", err)
	}

	headers := records[0]
	headerMap := map[string]int{}
	for i, h := range headers {
		headerMap[h] = i
	}

	typ := reflect.TypeOf((*T)(nil)).Elem()

	if typ.Kind() == reflect.Pointer {
		return nil, fmt.Errorf("type parameter T must be a struct, not a pointer to struct")
	}
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("type parameter T must be a struct")
	}

	fieldMap := map[string]int{}

	// Collect csv tags and fallback to field names
	// add test case it should only parse csv tag not any other
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("csv")
		csvKey := strings.Split(tag, ",")[0]
		if csvKey != "" {
			fieldMap[csvKey] = i
		}
	}

	// Check all required fields exist in CSV headers
	for key := range fieldMap {
		if _, ok := headerMap[key]; !ok {
			return nil, fmt.Errorf("missing required column: %s", key)
		}
	}

	var result []T
	for rowIndex, row := range records[1:] {
		entry := reflect.New(typ).Elem()

		for key, idx := range fieldMap {
			rawValue := ""
			colIndex, ok := headerMap[key]
			if ok && colIndex < len(row) {
				rawValue = strings.TrimSpace(row[colIndex])
			}

			field := entry.Field(idx)
			if !field.CanSet() {
				return nil, fmt.Errorf("cannot set field '%s'", key)
			}

			fieldType := field.Type()
			kind := fieldType.Kind()

			if kind == reflect.Pointer {
				elemKind := fieldType.Elem().Kind()

				if rawValue == "" {
					field.Set(reflect.Zero(fieldType))
					continue
				}

				ptrVal := reflect.New(fieldType.Elem())
				switch elemKind {
				case reflect.String:
					ptrVal.Elem().SetString(rawValue)
				case reflect.Int, reflect.Int64, reflect.Int32:
					intVal, err := strconv.Atoi(rawValue)
					if err != nil {
						return nil, fmt.Errorf("invalid int value for field '%s' at row %d: %v", key, rowIndex+2, err)
					}
					ptrVal.Elem().SetInt(int64(intVal))
				default:
					return nil, fmt.Errorf("unsupported pointer element type for field '%s'", key)
				}
				field.Set(ptrVal)
				continue
			}

			switch kind {
			case reflect.String:
				field.SetString(rawValue)
			case reflect.Int, reflect.Int64, reflect.Int32:
				if rawValue == "" {
					field.SetInt(0)
					continue
				}
				intVal, err := strconv.Atoi(rawValue)
				if err != nil {
					return nil, fmt.Errorf("invalid int value for field '%s' at row %d: %v", key, rowIndex+2, err)
				}
				field.SetInt(int64(intVal))
			default:
				return nil, fmt.Errorf("unsupported field type '%s' for field '%s'", kind.String(), key)
			}
		}
		result = append(result, entry.Interface().(T))
	}

	return result, nil
}
