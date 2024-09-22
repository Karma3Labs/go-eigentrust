package spopt

import "k3l.io/go-eigentrust/pkg/peer"

type Axis struct {
	Name    string
	PeerMap *peer.Map
	Alloc   bool // allocate indices to accommodate ids not in PeerMap
	Dim     int  // minimum (if grow) or fixed (if not grow) dimension
	Grow    bool // whether dimension can increase to match incoming indices
}

// Reset resets all axis options to their defaults.
//
// - Axis name is "i".
// - No PeerMap: Peer identifiers are parsed as integer literals.
// - No minimum Dim, axis starts from zero dimension.
// - Dim grows automatically to accommodate new indices.
func (o *Axis) Reset() {
	*o = Axis{}
	AxisName("i")(o)
	LiteralAxisIndices(o)
	MinAxisDim(0)(o)
}

// AxisName specifies an alternative axis name, e.g. "j" for the column axis.
func AxisName(name string) OptionForSet[Axis] {
	return func(o *Axis) { o.Name = name }
}

// LiteralAxisIndices causes peer identifiers to be parsed as integer indices.
func LiteralAxisIndices(o *Axis) { o.PeerMap, o.Alloc = nil, false }

// AxisIndicesIn causes peer identifiers to be looked up in a peer map,
// treating missing identifiers as errors.
func AxisIndicesIn(peerMap *peer.Map) OptionForSet[Axis] {
	return func(o *Axis) { o.PeerMap, o.Alloc = peerMap, false }
}

// AxisIndicesInto causes peer identifiers to be looked up in a peer map,
// allocating new indices for missing identifiers.
func AxisIndicesInto(peerMap *peer.Map) OptionForSet[Axis] {
	return func(o *Axis) { o.PeerMap, o.Alloc = peerMap, true }
}

// FixedAxisDim sets a fixed axis dimension;
// out-of-range indices are treated as errors.
func FixedAxisDim(dim int) OptionForSet[Axis] {
	return func(o *Axis) { o.Dim, o.Grow = dim, false }
}

// MinAxisDim sets a minimum axis dimension;
// the dimension grows to accommodate out-of-range indices.
func MinAxisDim(dim int) OptionForSet[Axis] {
	return func(o *Axis) { o.Dim, o.Grow = dim, true }
}
