// Copyright 2014 Apcera Inc. All rights reserved.

package deepmerge

import (
	"encoding/json"
	"errors"
	"reflect"
)

var NilDestinationError = errors.New("cannot use a nil map as a destination")

// Merge performs a deep merge of two maps, where it will copy the values in the
// src map into the dst map. Any values in the src map will overwrite the values
// in the dst map, except for values that are of type
// map[string]interface{}. This function is primarily intended for deep merging
// values from JSON, so it operates only on map[string]interface{} and not maps
// of other types. All other types are simply overwritten in the dst, including
// slices.
func Merge(dst, src map[string]interface{}) error {
	// check to see if the destination is nil
	if dst == nil {
		return NilDestinationError
	}

	// loop over the source to handle propagating it to the destination
	for key, srcValue := range src {
		dstValue, exists := dst[key]

		if exists {
			// handle if the key exists
			dstKind := reflect.ValueOf(dstValue).Kind()
			srcKind := reflect.ValueOf(dstValue).Kind()

			// if both types are a map, then recursively merge them
			if dstKind == reflect.Map && srcKind == reflect.Map {
				dstMap, dstOk := dstValue.(map[string]interface{})
				srcMap, srcOk := srcValue.(map[string]interface{})
				if dstOk && srcOk {
					// Ensure they're actually the right type, then recursively merge then
					// continue to the next item. If they are both not
					// map[string]interface{}, then it will fall through to the default of
					// overwriting.
					if err := Merge(dstMap, srcMap); err != nil {
						return err
					}
					continue
				}
			}

			// if we have reached this point, then simply overwrite the destination
			// with the source
			dstValue, err := uglyDeepCopy(srcValue)
			if err != nil {
				return err
			}
			dst[key] = dstValue

		} else {
			// if the key doesn't exist, simply set it directly
			dstValue, err := uglyDeepCopy(srcValue)
			if err != nil {
				return err
			}
			dst[key] = dstValue
		}
	}
	return nil
}

type uglyWrapper struct {
	Field interface{}
}

// uglyDeepCopy is a truly ugly and hacky way to ensure a deep copy is done on
// an object, but it is what is necessary to ensure that the source object is
// fully cloned so that it can be assigned to the destination and ensure futher
// changes within the destination do not use a shared pointer and alter the
// source. This method of wrapping it in JSON has side effects around integer
// handling in Go and limits object types, such has maps with only string
// keys. However, Go has no true deep copy functionality built in or currently
// available via third party packages that work or are up to date.
func uglyDeepCopy(v interface{}) (interface{}, error) {
	// marshal
	ugly := &uglyWrapper{Field: v}
	b, err := json.Marshal(ugly)
	if err != nil {
		return nil, err
	}

	// demarshal
	var ugly2 *uglyWrapper
	err = json.Unmarshal(b, &ugly2)
	if err != nil {
		return nil, err
	}

	return ugly2.Field, nil
}
