package basic

import "k3l.io/go-eigentrust/pkg/sparse"

// ComputeOpts contains options for the Compute function.
type ComputeOpts struct {
	t0             *sparse.Vector
	t              *sparse.Vector
	flatTailLength int
	numLeaders     int
	flatTailStats  *FlatTailStats
}

// ComputeOpt is one Compute option.
type ComputeOpt func(*ComputeOpts)

// WithInitialTrust (formerly the "t0" parameter)
// tells Compute to start iteration at the given trust vector
// instead of the pre-trust vector.
func WithInitialTrust(t0 *sparse.Vector) ComputeOpt {
	return func(o *ComputeOpts) { o.t0 = t0 }
}

// WithResultIn (formerly the "t" parameter).
// tells Compute to store the computation result in the given sparse vector
// without allocating one.
func WithResultIn(t *sparse.Vector) ComputeOpt {
	return func(o *ComputeOpts) { o.t = t }
}

// WithFlatTail (formerly the "flatTail" parameter)
// enables the flat-tail ranking stability check:
// It tells Compute to iterate until the trust ranking stabilizes,
// i.e. the same ranking is observed for l+1 iterations.
//
// This is in addition to the usual epsilon-based termination criterion:
// Iteration terminates only when both criteria are met.
// In order to use ranking stability only,
// pass Compute with e=1 to disable the epsilon-based convergence check.
func WithFlatTail(l int) ComputeOpt {
	return func(o *ComputeOpts) { o.flatTailLength = l }
}

// WithFlatTailNumLeaders (formerly the "numLeaders" parameter)
// tells Compute to limit trust ranking stability check to the top n peers.
func WithFlatTailNumLeaders(n int) ComputeOpt {
	return func(o *ComputeOpts) { o.numLeaders = n }
}

// WithFlatTailStats (formerly the "flatTailStats" parameter)
// tells Compute to populate the given struct with flat-tail algorithm stats
// upon completion.
func WithFlatTailStats(stats *FlatTailStats) ComputeOpt {
	return func(o *ComputeOpts) { o.flatTailStats = stats }
}
