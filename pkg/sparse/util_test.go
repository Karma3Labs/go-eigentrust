package sparse

import (
	"reflect"
	"testing"
)

func TestNilIfEmpty(t *testing.T) {
	type args[T any] struct {
		slice []T
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want []T
	}
	notEmptySlice := []int{3, 4}
	emptySlice := []int{}
	tests := []testCase[int]{
		{"NotEmpty", args[int]{notEmptySlice}, notEmptySlice},
		{"Empty", args[int]{emptySlice}, nil},
		{"Nil", args[int]{nil}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NilIfEmpty(tt.args.slice); !reflect.DeepEqual(got,
				tt.want) {
				t.Errorf("NilIfEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	type args[T any] struct {
		slice []T
		pred  func(T) bool
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want []T
	}
	tests := []testCase[int]{
		{
			"Positive",
			args[int]{
				[]int{3, 1, -4, -1, 5, 9, -2, -6, 5, 3, -5, -8, 9, 7},
				func(value int) bool { return value > 0 },
			},
			[]int{3, 1, 5, 9, 5, 3, 9, 7},
		},
		{
			"Negative",
			args[int]{
				[]int{3, 1, -4, -1, 5, 9, -2, -6, 5, 3, -5, -8, 9, 7},
				func(value int) bool { return value < 0 },
			},
			[]int{-4, -1, -2, -6, -5, -8},
		},
		{
			"Zero",
			args[int]{
				[]int{3, 1, -4, -1, 5, 9, -2, -6, 5, 3, -5, -8, 9, 7},
				func(value int) bool { return value == 0 },
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Filter(tt.args.slice,
				tt.args.pred); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}
