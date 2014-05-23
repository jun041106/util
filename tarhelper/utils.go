// Copyright 2014 Apcera Inc. All rights reserved.

package tarhelper

// defaultMappingFunc is the default mapping function when taring or untaring
// without specifying your own mapping function.
func defaultMappingFunc(id int) (int, error) {
	return id, nil
}
