package spopt

import "k3l.io/go-eigentrust/pkg/peer"

type Option = OptionForSet[Set]

var Noop = NoopForSet[Set]()

// WithOptions replace the current option set with the given one.
func WithOptions(options *Set) Option {
	return func(o *Set) {
		*o = *options
	}
}

func IndexNamed(name string) Option       { return func(o *Set) { AxisName(name)(o.Row) } }
func RowIndexNamed(name string) Option    { return func(o *Set) { AxisName(name)(o.Row) } }
func ColumnIndexNamed(name string) Option { return func(o *Set) { AxisName(name)(o.Column) } }
func ValueNamed(name string) Option       { return func(o *Set) { ValueName(name)(o.Value) } }

func LiteralIndices(o *Set)       { LiteralRowIndices(o); LiteralColumnIndices(o) }
func LiteralRowIndices(o *Set)    { LiteralAxisIndices(o.Row) }
func LiteralColumnIndices(o *Set) { LiteralAxisIndices(o.Column) }

func IndicesInto(m *peer.Map) Option {
	return func(o *Set) { RowIndicesInto(m)(o); ColumnIndicesInto(m)(o) }
}
func RowIndicesInto(m *peer.Map) Option    { return func(o *Set) { AxisIndicesInto(m)(o.Row) } }
func ColumnIndicesInto(m *peer.Map) Option { return func(o *Set) { AxisIndicesInto(m)(o.Column) } }

func IndicesIn(m *peer.Map) Option {
	return func(o *Set) { RowIndicesIn(m)(o); ColumnIndicesIn(m)(o) }
}
func RowIndicesIn(m *peer.Map) Option    { return func(o *Set) { AxisIndicesIn(m)(o.Row) } }
func ColumnIndicesIn(m *peer.Map) Option { return func(o *Set) { AxisIndicesIn(m)(o.Column) } }

func FixedDim(rows, columns int) Option {
	return func(o *Set) { FixedAxisDim(rows)(o.Row); FixedAxisDim(columns)(o.Column) }
}
func FixedRows(dim int) Option    { return func(o *Set) { FixedAxisDim(dim)(o.Row) } }
func FixedColumns(dim int) Option { return func(o *Set) { FixedAxisDim(dim)(o.Column) } }

func MinDim(rows, columns int) Option {
	return func(o *Set) { MinAxisDim(rows)(o.Row); MinAxisDim(columns)(o.Column) }
}
func MinRows(dim int) Option    { return func(o *Set) { MinAxisDim(dim)(o.Row) } }
func MinColumns(dim int) Option { return func(o *Set) { MinAxisDim(dim)(o.Column) } }

func IncludeZeroSetTo(include bool) Option {
	return func(o *Set) { IncludeZeroValueSetTo(include)(o.Value) }
}
func IncludeZero(o *Set) { IncludeZeroValue(o.Value) }
func ExcludeZero(o *Set) { ExcludeZeroValue(o.Value) }

func AllowNegativeSetTo(allow bool) Option {
	return func(o *Set) { AllowNegativeValueSetTo(allow)(o.Value) }
}
func AllowNegative(o *Set)    { AllowNegativeValue(o.Value) }
func DisallowNegative(o *Set) { DisallowNegativeValue(o.Value) }

func RowMajor(o *Set)    { o.ColumnMajor = false }
func ColumnMajor(o *Set) { o.ColumnMajor = true }
