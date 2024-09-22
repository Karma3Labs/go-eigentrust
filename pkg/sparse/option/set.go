package spopt

// New returns a new master option set with defaults + given options.
//
// See Set.Reset for the option defaults.
func New(opts ...Option) *Set { return newForSet[Set](opts...) }

// Set is the master set of sparse matrix/vector processing options.
type Set struct {
	Row         *Axis // also for vectors
	Column      *Axis
	Value       *Value
	ColumnMajor bool
}

// Reset resets all options to their defaults.
//
// - "i", "j", "v" for field names
// - Coordinate indices from external sources are assumed to be integer indices.
// - Dimensions have no minimum, and can grow to accommodate incoming indices.
// - Negative entries are not allowed.
// - Explicit zero entries are dropped (not included).
func (o *Set) Reset() {
	*o = Set{Row: &Axis{}, Column: &Axis{}, Value: &Value{}}
	resetAndApply(o.Row)
	resetAndApply(o.Column, AxisName("j"))
	resetAndApply(o.Value)
}

// RowColFromMajMin converts major-/minor-axis indices into row/column indices,
// depending on the ColumnMajor option.
// For use by lower-level routines that process sparse.CSMatrix directly.
func (o *Set) RowColFromMajMin() func(maj, min int) (row, col int) {
	if o.ColumnMajor {
		return func(maj, min int) (row, col int) { return min, maj }
	} else {
		return func(maj, min int) (row, col int) { return maj, min }
	}
}

// MajMinFromRowCol converts row/column indices into major-/minor-axis indices,
// depending on the ColumnMajor option.
// For use by lower-level routines that process sparse.CSMatrix directly.
func (o *Set) MajMinFromRowCol() func(row, col int) (maj, min int) {
	if o.ColumnMajor {
		return func(row, col int) (maj, min int) { return col, row }
	} else {
		return func(row, col int) (maj, min int) { return row, col }
	}
}

// MajorMinorAxes returns major/minor axes config,
// depending on the ColumnMajor option.
// For use by lower-level routines that process sparse.CSMatrix directly.
func (o *Set) MajorMinorAxes() (row, col *Axis) {
	if o.ColumnMajor {
		return o.Column, o.Row
	} else {
		return o.Row, o.Column
	}
}
