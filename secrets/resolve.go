// Copyright 2025 The Prometheus Authors
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

package secrets

import (
	"fmt"
	"reflect"
	"sort"
)

const (
	// maxRecursionDepth is the maximum depth for path traversals
	// in the config.
	maxRecursionDepth = 50
)

type fieldResults[T comparable] struct {
	paths map[T]string
	// ordered by increasing depth (root -> ... -> leafs)
	ordered []T
}

type fieldNode struct {
	path  string
	val   reflect.Value
	depth int
}

func (w *fieldNode) child(val reflect.Value, suffix string) fieldNode {
	return fieldNode{
		path:  w.path + suffix,
		val:   val,
		depth: w.depth + 1,
	}
}

func getTypeName(v interface{}) string {
	t := reflect.TypeOf(v)

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct {
		return t.Name()
	}
	return t.Kind().String()
}

func findFields[T comparable](v interface{}) (fieldResults[T], error) {
	results := fieldResults[T]{
		paths: make(map[T]string),
	}

	if v == nil || !reflect.ValueOf(v).IsValid() {
		return results, nil
	}
	if reflect.ValueOf(v).Kind() == reflect.Struct {
		return fieldResults[T]{}, fmt.Errorf("expected root to be pointer type, got struct %s instead", getTypeName(v))
	}
	visited := make(map[uintptr]bool)
	queue := []fieldNode{{path: getTypeName(v), val: reflect.ValueOf(v)}}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if node.depth > maxRecursionDepth {
			return fieldResults[T]{}, fmt.Errorf("path traversal exceeded maximum depth (current depth: %d):\n%v", node.depth, node.path)
		}

		queue = process(node, visited, queue)
		if node.val.CanInterface() {
			val := node.val
			if reflect.TypeOf((*T)(nil)).Elem().Kind() != reflect.Interface {
				if !val.CanAddr() {
					continue
				}
				val = val.Addr()
			}
			field, ok := val.Interface().(T)
			if !ok {
				continue
			}
			results.paths[field] = node.path
			results.ordered = append(results.ordered, field)
		}
	}
	return results, nil
}

func process(node fieldNode, visited map[uintptr]bool, queue []fieldNode) []fieldNode {
	val := node.val
	if !val.IsValid() {
		return queue
	}
	switch val.Kind() {
	case reflect.Ptr:
		if val.IsNil() || visited[val.Pointer()] {
			return queue
		}
		visited[val.Pointer()] = true
		return append(queue, node.child(val.Elem(), ""))
	case reflect.Interface:
		return append(queue, node.child(val.Elem(), ""))
	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			if field.CanInterface() {
				queue = append(queue, node.child(field, "."+val.Type().Field(i).Name))
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < val.Len(); i++ {
			queue = append(queue, node.child(val.Index(i), fmt.Sprintf("[%d]", i)))
		}
	case reflect.Map:
		keys := val.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprintf("%v", keys[i].Interface()) < fmt.Sprintf("%v", keys[j].Interface())
		})
		for i, key := range keys {
			queue = append(queue, node.child(key, fmt.Sprintf(".Keys()[%d]", i)))
			queue = append(queue, node.child(val.MapIndex(key), fmt.Sprintf("[%v]", key.Interface())))
		}
	}
	return queue
}
