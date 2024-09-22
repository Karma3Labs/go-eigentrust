package spopt

type resetter[O any] interface {
	*O
	Reset()
}

func newForSet[O any, P resetter[O]](opts ...OptionForSet[O]) *O {
	var o O
	var p P = &o
	p.Reset()
	apply(&o, opts...)
	return &o
}

func apply[O any](o *O, opts ...OptionForSet[O]) {
	for _, opt := range opts {
		opt(o)
	}
}

func resetAndApply[O any, P resetter[O]](o *O, opts ...OptionForSet[O]) {
	var p P = o
	p.Reset()
	apply(o, opts...)
}

// OptionForSet is a function that modifies the option set.
type OptionForSet[O any] func(*O)

// NoopForSet is a pseudo-option that changes nothing.
func NoopForSet[O any]() OptionForSet[O] {
	return func(*O) {}
}
