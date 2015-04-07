package lzma2

type control byte

// Constants for control bytes
const (
	// end of stream
	eosCtrl control = 0
	// copy content but reset dictionary
	copyResetDictCtrl = 0x01
	// copy content without resetting the dictionary
	copyCtrl = 0x02
	// mask for control bytes for a packed chunk
	packedMask = 0xe0
	// packed chunk; no update on state, properties or dictionary
	packedCtrl = 0x80
	// packed chunk; reset state
	packedResetStateCtrl = 0xa0
	// packed chunk; reset state, new properties
	packedNewPropsCtrl = 0xc0
	// packed chunk; reset state, new properties, reset dictionary
	packedResetDictCtrl = 0xe0
)

func (c control) eos() bool {
	return c == eosCtrl
}

func (c control) packed() bool {
	return c&packedCtrl == packedCtrl
}

func (c control) resetDict() bool {
	if !c.packed() {
		return c == copyResetDictCtrl
	}
	return (c & packedMask) == packedResetDictCtrl
}

func (c control) resetState() bool {
	if !c.packed() {
		return false
	}
	return (c & packedMask) >= packedResetStateCtrl
}

func (c control) newProps() bool {
	if !c.packed() {
		return false
	}
	return (c & packedMask) >= packedNewPropsCtrl
}

func (c control) unpackedSizeHighBits() int64 {
	if !c.packed() {
		return 0
	}
	return int64(c&^packedMask) << 16
}
