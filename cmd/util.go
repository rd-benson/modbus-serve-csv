package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Euclid's algorithm to find greatest common divisor
func GCD(p, q uint16) uint16 {
	if q == 0 {
		return p
	}

	r := p % q
	return GCD(q, r)
}

// GCD algorithm applied to a list
func GCDSlice(slice []uint16) uint16 {
	x, y, z := slice[0], slice[1], slice[2:]
	r := GCD(x, y)
	if len(z) == 0 {
		return r
	}
	s := append(z, r)
	return GCDSlice(s)
}

// Test if a slice contains an element
func contains[T comparable](testElem T, slice []T) bool {
	for _, sliceElem := range slice {
		if testElem == sliceElem {
			return true
		}
	}
	return false
}

// Find files of type ext in root directory only
func findByExt(root, ext string) []string {
	var a []string
	dirEntries, err := os.ReadDir(root)
	cobra.CheckErr(err)

	for _, d := range dirEntries {
		if filepath.Ext(d.Name()) == ext {
			a = append(a, filepath.Base(d.Name()))
		}
	}

	return a
}

// Find slice min/max
func minSliceUint(array []uint16) uint16 {
	min := array[0]
	for _, value := range array {
		if min > value {
			min = value
		}
	}
	return min
}
