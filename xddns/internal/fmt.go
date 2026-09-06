package internal

import (
	"strconv"
	"strings"
)

// JoinInts joins a slice of integers into a single string, separated by the specified separator.
func JoinInts(nums []int, sep string) string {
	strs := make([]string, len(nums))
	for idx, n := range nums {
		strs[idx] = strconv.Itoa(n)
	}

	return strings.Join(strs, sep)
}
