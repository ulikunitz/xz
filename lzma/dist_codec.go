package lzma

// Constants used by the distance codec.
const (
	// minimum supported distance
	minDistance = 1
	// maximum supported distance
	maxDistance = 1 << 32
	// number of the supported len states
	lenStates = 4
	// start for the position models
	startPosModel = 4
	// first index with align bits support
	endPosModel = 14
	// bits for the position slots
	posSlotBits = 6
	// number of align bits
	alignBits = 4
	// maximum positon slot
	maxPosSlot = 63
)

// distCodec provides encoding and decoding of distance values.
type distCodec struct {
	posSlotCodecs [lenStates]treeCodec
	posModel      [endPosModel - startPosModel]treeReverseCodec
	alignCodec    treeReverseCodec
}

// newDistCodec creates a new distance codec.
func newDistCodec() *distCodec {
	dc := new(distCodec)
	for i := range dc.posSlotCodecs {
		dc.posSlotCodecs[i] = makeTreeCodec(posSlotBits)
	}
	for i := range dc.posModel {
		posSlot := startPosModel + i
		bits := (posSlot >> 1) - 1
		dc.posModel[i] = makeTreeReverseCodec(bits)
	}
	dc.alignCodec = makeTreeReverseCodec(alignBits)
	return dc
}

// Converts the value l to a supported lenState value.
func lenState(l uint32) uint32 {
	if l >= lenStates {
		l = lenStates - 1
	}
	return l
}

// Encode encodes the distance using the parameter l. Dist can have values from
// the full range of uint32 values. To get the distance offset the actual match
// distance has to be decreased by 1. A distance offset of 0xffffffff (eos)
// indicates the end of the stream.
func (dc *distCodec) Encode(dist uint32, l uint32, e *rangeEncoder,
) (err error) {
	// Compute the posSlot using nlz32
	var posSlot uint32
	var bits uint32
	if dist < startPosModel {
		posSlot = dist
	} else {
		bits = uint32(30 - nlz32(dist))
		posSlot = startPosModel - 2 + (bits << 1)
		posSlot += (dist >> uint(bits)) & 1
	}

	if err = dc.posSlotCodecs[lenState(l)].Encode(posSlot, e); err != nil {
		return
	}

	switch {
	case posSlot < startPosModel:
		return nil
	case posSlot < endPosModel:
		tc := &dc.posModel[posSlot-startPosModel]
		return tc.Encode(dist, e)
	}
	dic := directCodec(bits - alignBits)
	if err = dic.Encode(dist>>alignBits, e); err != nil {
		return
	}
	return dc.alignCodec.Encode(dist, e)
}

// Decode decodes the distance offset using the parameter l. The dist value
// 0xffffffff (eos) indicates the end of the stream. Add one to the distance
// offset to get the actual match distance.
func (dc *distCodec) Decode(l uint32, d *rangeDecoder,
) (dist uint32, err error) {
	posSlot, err := dc.posSlotCodecs[lenState(l)].Decode(d)
	if err != nil {
		return
	}

	// posSlot equals distance
	if posSlot < startPosModel {
		return posSlot, nil
	}

	// posSlot are using the individial models
	bits := (posSlot >> 1) - 1
	dist = (2 | (posSlot & 1)) << bits
	var u uint32
	if posSlot < endPosModel {
		tc := &dc.posModel[posSlot-startPosModel]
		if u, err = tc.Decode(d); err != nil {
			return 0, err
		}
		dist += u
		return dist, nil
	}

	// posSlots use direct encoding and a single model for the four align
	// bits.
	dic := directCodec(bits - alignBits)
	if u, err = dic.Decode(d); err != nil {
		return 0, err
	}
	dist += u << alignBits
	if u, err = dc.alignCodec.Decode(d); err != nil {
		return 0, err
	}
	dist += u
	return dist, nil
}
