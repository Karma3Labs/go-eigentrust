package spopt

// Value is the set of options that apply to entry values.
type Value struct {
	Name          string
	AllowNegative bool
	IncludeZero   bool
}

func (o *Value) Reset() {
	*o = Value{}
	ValueName("v")(o)
	DisallowNegativeValue(o)
	ExcludeZeroValue(o)
}

func ValueName(name string) OptionForSet[Value] {
	return func(o *Value) { o.Name = name }
}

func AllowNegativeValueSetTo(allow bool) OptionForSet[Value] {
	return func(o *Value) { o.AllowNegative = allow }
}

func IncludeZeroValueSetTo(include bool) OptionForSet[Value] {
	return func(o *Value) { o.IncludeZero = include }
}

func AllowNegativeValue(o *Value)    { o.AllowNegative = true }
func DisallowNegativeValue(o *Value) { o.AllowNegative = false }
func IncludeZeroValue(o *Value)      { o.IncludeZero = true }
func ExcludeZeroValue(o *Value)      { o.IncludeZero = false }
