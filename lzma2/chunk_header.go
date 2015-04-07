package lzma2

import (
	"io"

	"github.com/uli-go/xz/lzbase"
)

const (
	minUnpackedSize = 1
	maxUnpackedSize = 1 << 21
	minPackedSize   = 1
	maxPackedSize   = 1 << 16
)

// control represents the control byte of the chunk header
type control byte

// Constants for control bytes
const (
	// end of stream
	eosCtrl control = 0
	// mask for copy relevant controls
	copyMask = 0x03
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

// eos returns whether the control marks the end of the stream
func (c control) eos() bool {
	return c == eosCtrl
}

func verifyControl(c control) error {
	if c.packed() {
		return nil
	}
	if c&^copyMask != 0 {
		return newError("control has invalid value")
	}
	return nil
}

// packed returns whether the control indicates a packed chunk
func (c control) packed() bool {
	return c&packedCtrl == packedCtrl
}

// resetDict indicates whether the control requires a reset of a
// dictionary
func (c control) resetDict() bool {
	if !c.packed() {
		return c == copyResetDictCtrl
	}
	return (c & packedMask) == packedResetDictCtrl
}

// resetState indicates whether a reset of the encoder state is required
func (c control) resetState() bool {
	if !c.packed() {
		return false
	}
	return (c & packedMask) >= packedResetStateCtrl
}

// newProps indicates whether new properties are required
func (c control) newProps() bool {
	if !c.packed() {
		return false
	}
	return (c & packedMask) >= packedNewPropsCtrl
}

// unpackedSizeHighBits returns the high bits of the unpacked size at the right
// positon of the returned value.
func (c control) unpackedSizeHighBits() int64 {
	if !c.packed() {
		return 0
	}
	return int64(c&^packedMask) << 16
}

type chunkHeader struct {
	control
	packedSize   int64
	unpackedSize int64
	props        lzbase.Properties
}

func computeControl(h chunkHeader) control {
	c := h.control
	if !c.packed() {
		return c
	}
	u := control((h.unpackedSize-1)>>16) &^ packedMask
	return (c & packedMask) | u
}

func verifyChunkHeader(h chunkHeader) error {
	var err error
	if err = verifyControl(h.control); err != nil {
		return err
	}
	if !(minPackedSize <= h.packedSize && h.packedSize <= maxPackedSize) {
		return newError("packedSize out of range")
	}
	if !(minUnpackedSize <= h.unpackedSize &&
		h.unpackedSize <= maxUnpackedSize) {
		return newError("unpackedSize out of range")
	}
	return verifyProperties(h.props.LC(), h.props.LP(), h.props.PB())
}

func writeChunkHeader(w io.Writer, h chunkHeader) (n int, err error) {
	if err = verifyChunkHeader(h); err != nil {
		return 0, err
	}
	buf := make([]byte, 1, 5)
	c := computeControl(h)
	buf[0] = byte(c)
	panic("TODO")

}
