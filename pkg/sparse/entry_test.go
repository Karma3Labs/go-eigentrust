package sparse

import (
	"reflect"
	"testing"
)

func TestCSREntriesSort_Len(t *testing.T) {
	tests := []struct {
		name string
		a    CSREntriesSort
		want int
	}{
		{
			"Normal",
			CSREntriesSort([]CooEntry{
				{3, 1, 7.0},
				{1, 0, 4.0},
				{2, 8, 0.0},
				{5, 0, 0.0},
			}),
			4,
		},
		{
			"Empty",
			CSREntriesSort([]CooEntry{}),
			0,
		},
		{
			"Nil",
			nil,
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Len(); got != tt.want {
				t.Errorf("a.Len() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCSREntriesSort_Swap(t *testing.T) {
	type args struct {
		i int
		j int
	}
	tests := []struct {
		name string
		a    CSREntriesSort
		args args
		want CSREntriesSort
	}{
		{
			"Normal",
			[]CooEntry{
				{3, 1, 7.0},
				{1, 0, 4.0},
				{2, 8, 0.0},
				{5, 0, 0.0},
			},
			args{1, 2},
			[]CooEntry{
				{3, 1, 7.0},
				{2, 8, 0.0},
				{1, 0, 4.0},
				{5, 0, 0.0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.a.Swap(tt.args.i, tt.args.j)
		})
		if !reflect.DeepEqual(tt.a, tt.want) {
			t.Errorf("a = %v, want %v", tt.a, tt.want)
		}
	}
}

func TestCSREntriesSort_Less(t *testing.T) {
	tests := []struct {
		name string
		x, y CooEntry
		want bool
	}{
		{"xr<yr,xc<yc", CooEntry{0, 0, 0}, CooEntry{1, 1, 0}, true},
		{"xr<yr,xc=yc", CooEntry{0, 1, 0}, CooEntry{1, 1, 0}, true},
		{"xr<yr,xc>yc", CooEntry{0, 2, 0}, CooEntry{1, 1, 0}, true},
		{"xr=yr,xc<yc", CooEntry{1, 0, 0}, CooEntry{1, 1, 0}, true},
		{"xr=yr,xc=yc", CooEntry{1, 1, 0}, CooEntry{1, 1, 0}, false},
		{"xr=yr,xc>yc", CooEntry{1, 2, 0}, CooEntry{1, 1, 0}, false},
		{"xr>yr,xc<yc", CooEntry{2, 0, 0}, CooEntry{1, 1, 0}, false},
		{"xr>yr,xc=yc", CooEntry{2, 1, 0}, CooEntry{1, 1, 0}, false},
		{"xr>yr,xc>yc", CooEntry{2, 2, 0}, CooEntry{1, 1, 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := CSREntriesSort([]CooEntry{tt.x, tt.y})
			if got := a.Less(0, 1); got != tt.want {
				t.Errorf("a.Less(%v, %v) = %v, want %v",
					tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestCSCEntriesSort_Len(t *testing.T) {
	tests := []struct {
		name string
		a    CSCEntriesSort
		want int
	}{
		{
			"Normal",
			CSCEntriesSort([]CooEntry{
				{3, 1, 7.0},
				{1, 0, 4.0},
				{2, 8, 0.0},
				{5, 0, 0.0},
			}),
			4,
		},
		{
			"Empty",
			CSCEntriesSort([]CooEntry{}),
			0,
		},
		{
			"Nil",
			nil,
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Len(); got != tt.want {
				t.Errorf("a.Len() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCSCEntriesSort_Swap(t *testing.T) {
	type args struct {
		i int
		j int
	}
	tests := []struct {
		name string
		a    CSCEntriesSort
		args args
		want CSCEntriesSort
	}{
		{
			"Normal",
			[]CooEntry{
				{3, 1, 7.0},
				{1, 0, 4.0},
				{2, 8, 0.0},
				{5, 0, 0.0},
			},
			args{1, 2},
			[]CooEntry{
				{3, 1, 7.0},
				{2, 8, 0.0},
				{1, 0, 4.0},
				{5, 0, 0.0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.a.Swap(tt.args.i, tt.args.j)
		})
		if !reflect.DeepEqual(tt.a, tt.want) {
			t.Errorf("a = %v, want %v", tt.a, tt.want)
		}
	}
}

func TestCSCEntriesSort_Less(t *testing.T) {
	tests := []struct {
		name string
		x, y CooEntry
		want bool
	}{
		{"xr<yr,xc<yc", CooEntry{0, 0, 0}, CooEntry{1, 1, 0}, true},
		{"xr=yr,xc<yc", CooEntry{1, 0, 0}, CooEntry{1, 1, 0}, true},
		{"xr>yr,xc<yc", CooEntry{2, 0, 0}, CooEntry{1, 1, 0}, true},
		{"xr<yr,xc=yc", CooEntry{0, 1, 0}, CooEntry{1, 1, 0}, true},
		{"xr=yr,xc=yc", CooEntry{1, 1, 0}, CooEntry{1, 1, 0}, false},
		{"xr>yr,xc=yc", CooEntry{2, 1, 0}, CooEntry{1, 1, 0}, false},
		{"xr<yr,xc>yc", CooEntry{0, 2, 0}, CooEntry{1, 1, 0}, false},
		{"xr=yr,xc>yc", CooEntry{1, 2, 0}, CooEntry{1, 1, 0}, false},
		{"xr>yr,xc>yc", CooEntry{2, 2, 0}, CooEntry{1, 1, 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := CSCEntriesSort([]CooEntry{tt.x, tt.y})
			if got := a.Less(0, 1); got != tt.want {
				t.Errorf("a.Less(%v, %v) = %v, want %v",
					tt.x, tt.y, got, tt.want)
			}
		})
	}
}
