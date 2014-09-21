package lzma

const (
	lenStates     = 4
	startPosModel = 4
	endPosModel   = 14
	posSlotBits   = 6
	alignBits     = 4
	maxPosSlot    = 63
)

type distCodec struct {
	posSlotCodecs [lenStates]treeCodec
	posModel      [endPosModel - startPosModel]treeReverseCodec
	alignCodec    treeReverseCodec
}

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

func lenState(l uint32) uint32 {
	s := l
	if s >= lenStates {
		s = lenStates - 1
	}
	return s
}

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

func (dc *distCodec) Decode(l uint32, d *rangeDecoder,
) (dist uint32, err error) {
	posSlot, err := dc.posSlotCodecs[lenState(l)].Decode(d)
	if err != nil {
		return
	}
	if posSlot < startPosModel {
		return posSlot, nil
	}

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
