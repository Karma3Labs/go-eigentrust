package basic

import (
	"reflect"
	"testing"

	"k3l.io/go-eigentrust/pkg/sparse"
)

func TestExtractDistrust(t *testing.T) {
	type args struct {
		localTrust *sparse.Matrix
	}
	tests := []struct {
		name         string
		args         args
		wantTrust    *sparse.Matrix
		wantDistrust *sparse.Matrix
		wantErr      bool
	}{
		{
			name: "test1",
			args: args{
				localTrust: &sparse.Matrix{
					CSMatrix: sparse.CSMatrix{
						MajorDim: 3,
						MinorDim: 3,
						Entries: [][]sparse.Entry{
							/* 0 */ {
								{Index: 0, Value: 100},
								{Index: 1, Value: -50},
								{Index: 2, Value: -50},
							},
							/* 1 */ nil,
							/* 2 */ {{Index: 0, Value: -100}},
						},
					},
				},
			},
			wantTrust: &sparse.Matrix{
				CSMatrix: sparse.CSMatrix{
					MajorDim: 3,
					MinorDim: 3,
					Entries: [][]sparse.Entry{
						/* 0 */ {{Index: 0, Value: 100}},
						/* 1 */ nil,
						/* 2 */ nil,
					},
				},
			},
			wantDistrust: &sparse.Matrix{
				CSMatrix: sparse.CSMatrix{
					MajorDim: 3,
					MinorDim: 3,
					Entries: [][]sparse.Entry{
						/* 0 */ {{Index: 1, Value: 50}, {Index: 2, Value: 50}},
						/* 1 */ nil,
						/* 2 */ {{Index: 0, Value: 100}},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractDistrust(tt.args.localTrust)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractDistrust() error = %v, wantErr %v", err,
					tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.args.localTrust, tt.wantTrust) {
				t.Errorf("ExtractDistrust() got trust = %v, want %v",
					tt.args.localTrust, tt.wantTrust)
			}
			if !reflect.DeepEqual(got, tt.wantDistrust) {
				t.Errorf("ExtractDistrust() got distrust = %v, want %v",
					got, tt.wantDistrust)
			}
		})
	}
}
