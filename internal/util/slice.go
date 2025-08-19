package util

// IntersectIndices returns the intersection of two integer slices
// It returns elements that exist in both slices a and b
func IntersectIndices(a, b []int) []int {
	if len(a) == 0 || len(b) == 0 {
		return []int{}
	}
	
	setA := make(map[int]bool)
	for _, v := range a {
		setA[v] = true
	}
	
	var result []int
	for _, v := range b {
		if setA[v] {
			result = append(result, v)
		}
	}
	
	if result == nil {
		return []int{}
	}
	return result
}