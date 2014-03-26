// Copyright 2014 Apcera Inc. All rights reserved.

package deepmerge

import (
	"reflect"
)

// Merge performs a deep merge of two maps, where it will copy the values in the
// src map into the dst map. Any values in the src map will overwrite the values
// in the dst map, except for values that are of type
// map[string]interface{}. This function is primarily intended for deep merging
// values from JSON, so it operates only on map[string]interface{} and not maps
// of other types. All other types are simply overwritten in the dst, including
// slices.
func Merge(dst, src map[string]interface{}) {
	// loop over the source to handle propagating it to the destination
	for key, srcValue := range src {
		dstValue, exists := dst[key]

		if exists {
			// handle if the key exists
			dstKind := reflect.ValueOf(dstValue).Kind()
			srcKind := reflect.ValueOf(dstValue).Kind()

			// if both types are a map, then recursively merge them
			if dstKind == reflect.Map && srcKind == reflect.Map {
				dstMap := dstValue.(map[string]interface{})
				srcMap := srcValue.(map[string]interface{})
				if dstMap != nil && srcMap != nil {
					// Ensure they're actually the right type, then recursively merge then
					// continue to the next item. If they are both not
					// map[string]interface{}, then it will fall through to the default of
					// overwriting.
					Merge(dstMap, srcMap)
					continue
				}
			}

			// if we have reached this point, then simply overwrite the destination
			// with the source
			dst[key] = srcValue

		} else {
			// if the key doesn't exist, simply set it directly
			dst[key] = srcValue
		}
	}
}
