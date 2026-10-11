package test

import "testing"

func TestLengthOfLIS(t *testing.T) {
	arr := []int{10, 9, 2, 5, 3, 7, 101, 18}
	got := lengthOfLIS(arr)
	t.Logf("got = %d", got)

	// arr1 := []int{0, 1, 0, 3, 2, 3}
	// got1 := lengthOfLIS(arr1)
	// t.Logf("got = %d", got1)

	// arr2 := []int{7, 7, 7, 7, 7, 7, 7}
	// got2 := lengthOfLIS(arr2)
	// t.Logf("got = %d", got2)
}
