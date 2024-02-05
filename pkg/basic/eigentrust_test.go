package basic

import (
	"testing"

	"github.com/magiconair/properties/assert"
	"k3l.io/go-eigentrust/pkg/sparse"
)

func TestDiscountTrustVector(t *testing.T) {
	type args struct {
		t         *sparse.Vector
		discounts *sparse.Matrix
	}
	tests := []struct {
		name     string
		args     args
		expected *sparse.Vector
	}{
		{
			"test1",
			args{
				t: &sparse.Vector{
					Dim: 5,
					Entries: []sparse.Entry{
						{0, 0.25},
						{2, 0.5},
						{3, 0.25},
					},
				},
				discounts: &sparse.Matrix{
					CSMatrix: sparse.CSMatrix{
						MajorDim: 5,
						MinorDim: 5,
						Entries: [][]sparse.Entry{
							// 0 - no distrust
							{},
							// 1 - doesn't matter because of zero trust
							{
								{2, 0.5},
								{3, 0.5},
							},
							// 2 - scaled by 0.5 and applied
							{
								{0, 0.25},
								{4, 0.75},
							},
							// 3 - scaled by 0.25 and applied
							{
								{2, 0.5},
								{4, 0.5},
							},
							// 4 - no distrust, also zero global trust
							{},
						},
					},
				},
			},
			&sparse.Vector{
				Dim: 5,
				Entries: []sparse.Entry{
					// {index, original - distrust*gt}
					{0, 0.25 - 0.25*0.5 /* peer 2 */},
					{2, 0.5 - 0.5*0.25 /* peer 3 */},
					{3, 0.25},
					{4, 0 - 0.75*0.5 /* peer 2 */ - 0.5*0.25 /* peer 3 */},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DiscountTrustVector(tt.args.t, tt.args.discounts)
			assert.Equal(t, tt.args.t, tt.expected)
		})
	}
}
