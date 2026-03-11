//go:build !solution

package tour1

func Tour() string {
	return "tour1 done!"
}

func searchInsert(nums []int, target int) int {
	l, r := 0, len(nums) - 1
	i := (l + r) / 2
	for l != r {
		if target == nums[i] {
			break
		} else if target < nums[i] {
			r = i
		} else {
			l = i
		}
		i = (l + r) / 2
	}

	return i
}

func main() {
	fmt.Println("%v", searchInsert([1,2,3,4,5], 4))
}
