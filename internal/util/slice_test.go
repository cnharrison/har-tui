package util

import (
	"reflect"
	"testing"
)

func TestIntersectIndices(t *testing.T) {
	tests := []struct {
		name     string
		a        []int
		b        []int
		expected []int
	}{
		{
			name:     "basic intersection",
			a:        []int{1, 2, 3, 4, 5},
			b:        []int{3, 4, 5, 6, 7},
			expected: []int{3, 4, 5},
		},
		{
			name:     "no intersection",
			a:        []int{1, 2, 3},
			b:        []int{4, 5, 6},
			expected: []int{},
		},
		{
			name:     "identical slices",
			a:        []int{1, 2, 3},
			b:        []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "empty first slice",
			a:        []int{},
			b:        []int{1, 2, 3},
			expected: []int{},
		},
		{
			name:     "empty second slice",
			a:        []int{1, 2, 3},
			b:        []int{},
			expected: []int{},
		},
		{
			name:     "both empty",
			a:        []int{},
			b:        []int{},
			expected: []int{},
		},
		{
			name:     "duplicates in first slice",
			a:        []int{1, 1, 2, 2, 3},
			b:        []int{2, 3, 4},
			expected: []int{2, 3},
		},
		{
			name:     "duplicates in second slice",
			a:        []int{1, 2, 3},
			b:        []int{2, 2, 3, 3, 4},
			expected: []int{2, 2, 3, 3}, // Preserves duplicates from b
		},
		{
			name:     "single element intersection",
			a:        []int{5},
			b:        []int{1, 2, 3, 5, 6},
			expected: []int{5},
		},
		{
			name:     "unordered slices",
			a:        []int{5, 1, 3, 2, 4},
			b:        []int{6, 2, 4, 1, 7},
			expected: []int{2, 4, 1}, // Order follows b
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IntersectIndices(tt.a, tt.b)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("IntersectIndices(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func BenchmarkIntersectIndices(b *testing.B) {
	a := make([]int, 1000)
	sliceB := make([]int, 1000)
	
	for i := 0; i < 1000; i++ {
		a[i] = i
		sliceB[i] = i + 500 // 50% overlap
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IntersectIndices(a, sliceB)
	}
}