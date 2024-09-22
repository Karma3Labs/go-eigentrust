package basic

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
						{Value: 0.25},
						{Index: 2, Value: 0.5},
						{Index: 3, Value: 0.25},
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
								{Index: 2, Value: 0.5},
								{Index: 3, Value: 0.5},
							},
							// 2 - scaled by 0.5 and applied
							{
								{Index: 0, Value: 0.25},
								{Index: 4, Value: 0.75},
							},
							// 3 - scaled by 0.25 and applied
							{
								{Index: 2, Value: 0.5},
								{Index: 4, Value: 0.5},
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
					{Index: 0, Value: 0.25 - 0.25*0.5 /* peer 2 */},
					{Index: 2, Value: 0.5 - 0.5*0.25 /* peer 3 */},
					{Index: 3, Value: 0.25},
					{
						Index: 4,
						Value: 0 - 0.75*0.5 /* peer 2 */ - 0.5*0.25, /* peer 3 */
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DiscountTrustVector(tt.args.t, tt.args.discounts)
			assert.Nil(t, err)
			assert.Equal(t, tt.args.t, tt.expected)
		})
	}
}
