package sparse

import (
	"context"
	"reflect"
	"testing"
)

func TestCSMatrix_Transpose(t *testing.T) {
	tests := []struct {
		name       string
		original   *CSMatrix
		transposed *CSMatrix
	}{
		{
			name: "Normal",
			//   ║   0    1    2    3
			// ══╬═══════════════════
			// 0 ║ 100  200  300    0
			// 1 ║   0  400    0  500
			// 2 ║   0    0    0    0
			// 3 ║ 600  700  800  900
			// 4 ║   0    0 1000    0
			original: &CSMatrix{
				MajorDim: 5,
				MinorDim: 4,
				Entries: [][]Entry{
					{{0, 100}, {1, 200}, {2, 300}},
					{{1, 400}, {3, 500}},
					nil,
					{{0, 600}, {1, 700}, {2, 800}, {3, 900}},
					{{2, 1000}},
				},
			},
			transposed: &CSMatrix{
				MajorDim: 4,
				MinorDim: 5,
				Entries: [][]Entry{
					{{0, 100}, {3, 600}},
					{{0, 200}, {1, 400}, {3, 700}},
					{{0, 300}, {3, 800}, {4, 1000}},
					{{1, 500}, {3, 900}},
				},
			},
		},
		{
			name: "Empty",
			original: &CSMatrix{
				MajorDim: 5,
				MinorDim: 3,
				Entries:  [][]Entry{nil, nil, nil, nil, nil},
			},
			transposed: &CSMatrix{
				MajorDim: 3,
				MinorDim: 5,
				Entries:  [][]Entry{nil, nil, nil},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, _ := tt.original.Transpose(context.Background())
			if !reflect.DeepEqual(mt, tt.transposed) {
				t.Errorf("original.Transpose() = %v, want %v", mt,
					tt.transposed)
			}
			mtt, _ := mt.Transpose(context.Background())
			if !reflect.DeepEqual(mtt, tt.original) {
				t.Errorf("original.Transpose().Transpose() = %v, want %v", mtt,
					tt.original)
			}
		})
	}
}

func TestNewCSRMatrix(t *testing.T) {
	type args struct {
		rows, cols int
		entries    []CooEntry
	}
	tests := []struct {
		name string
		args args
		want *CSRMatrix
	}{
		{
			name: "Empty",
			args: args{0, 0, nil},
			want: &CSRMatrix{
				CSMatrix: CSMatrix{
					MajorDim: 0,
					MinorDim: 0,
					Entries:  nil,
				},
			},
		},
		//   ║   0    1    2    3
		// ══╬═══════════════════
		// 0 ║ 100  200  300    0
		// 1 ║   0  400    0  500
		// 2 ║   0    0    0    0
		// 3 ║ 600  700  800  900
		// 4 ║   0    0 1000    0
		{
			name: "Normal",
			args: args{
				5, 4,
				[]CooEntry{
					{0, 0, 100},
					{3, 0, 600},
					{3, 1, 700},
					{1, 1, 400},
					{0, 1, 200},
					{2, 1, 0}, // zero value should be dropped
					{1, 3, 500},
					{3, 3, 900},
					{4, 2, 1000},
					{0, 2, 300},
					{3, 2, 800},
				},
			},
			want: &CSRMatrix{
				CSMatrix: CSMatrix{
					MajorDim: 5,
					MinorDim: 4,
					Entries: [][]Entry{
						{{0, 100}, {1, 200}, {2, 300}},
						{{1, 400}, {3, 500}},
						nil,
						{{0, 600}, {1, 700}, {2, 800}, {3, 900}},
						{{2, 1000}},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewCSRMatrix(
				tt.args.rows, tt.args.cols, tt.args.entries,
			); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewCSRMatrix() = %v, want %v", got, tt.want)
			}
		})
	}
}
