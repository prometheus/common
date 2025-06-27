package secrets

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

var secretFieldType = reflect.TypeOf(SecretField{})

type SecretPaths map[string]SecretField

type walkItem struct {
	path []string
	val  reflect.Value
}

func getSecretFields(v interface{}) (SecretPaths, error) {
	results := make(SecretPaths)
	if v == nil {
		return results, nil
	}
	visited := make(map[uintptr]bool)
	queue := []walkItem{{path: nil, val: reflect.ValueOf(v)}}

	for len(queue) > 0 {
		currentItem := queue[0]
		queue = queue[1:]

		path := currentItem.path
		val := currentItem.val
		if len(path) > 50 {
			return nil, fmt.Errorf("path traversal exceeded maximum depth (current depth: %d)", len(path))
		}

		if val.Type() == secretFieldType {
			path := strings.Join(path, ".")
			secret, ok := val.Interface().(SecretField)
			if !ok {
				return nil, fmt.Errorf("path '%s': internal error: matched SecretField type but failed type assertion", path)
			}
			results[path] = secret
			continue
		}
		queue = process(path, val, visited, queue)
	}
	return results, nil
}

func process(path []string, val reflect.Value, visited map[uintptr]bool, queue []walkItem) []walkItem {
	if !val.IsValid() {
		return queue
	}
	switch val.Kind() {
	case reflect.Ptr:
		if val.IsNil() || visited[val.Pointer()] {
			return queue
		}
		visited[val.Pointer()] = true
		return append(queue, walkItem{path: path, val: val.Elem()})
	case reflect.Interface:
		return append(queue, walkItem{path: path, val: val.Elem()})
	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			newPath := append(path, val.Type().Field(i).Name)
			field := val.Field(i)
			if field.CanInterface() {
				queue = append(queue, walkItem{path: newPath, val: field})
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < val.Len(); i++ {
			newPath := append(path, fmt.Sprintf("[%d]", i))
			queue = append(queue, walkItem{path: newPath, val: val.Index(i)})
		}
	case reflect.Map:
		keys := val.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprintf("%v", keys[i].Interface()) < fmt.Sprintf("%v", keys[j].Interface())
		})
		for _, key := range keys {
			keyPath := append(path, fmt.Sprintf("[%v:key]", key.Interface()))
			queue = append(queue, walkItem{path: keyPath, val: key})
			valPath := append(path, fmt.Sprintf("[%v]", key.Interface()))
			queue = append(queue, walkItem{path: valPath, val: val.MapIndex(key)})
		}
	}
	return queue
}
