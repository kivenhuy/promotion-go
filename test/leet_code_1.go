package test

import (
	"fmt"
	"slices"
)

func lengthOfLIS(nums []int) int {
	lengthNum := len(nums)
	fmt.Println("Nums =", nums)
	dp := make([]int, lengthNum)

	// for i := range lengthNum {
	// 	if i != 0 && i+1 < len(nums) && nums[i-1] > nums[i] {
	// 		fmt.Println("first ", nums[i-1])
	// 		fmt.Println("after ", nums[i])
	// 		newNums = slices.Delete(nums, i, i+1)
	// 	}
	// }

	for i := range lengthNum {
		dp[i] = 1
		// fmt.Println("i ", i)
		// if i != 0 {
		for j := range i {
			// fmt.Println("i ,j  ", i,j)
			// fmt.Println("dp[i] , dp[j]  ", dp[i] , dp[j])
			// fmt.Println("nums ", nums[j], nums[i])
			if nums[j] < nums[i] {
				fmt.Println("i ", i,j)
				dp[i] = max(dp[i], dp[j]+1)
				fmt.Println("dp[i] ", dp)
				
			}
		}
		// }
	}
	return slices.Max(dp)
}
