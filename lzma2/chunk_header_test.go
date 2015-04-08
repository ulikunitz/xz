package lzma2

import (
	"bytes"
	"testing"
)

func TestChunkHeaderHandling(t *testing.T) {
	props := Default.Properties()
	tests := []chunkHeader{
		chunkHeader{control: eosCtrl},
		chunkHeader{control: copyCtrl, unpackedSize: minUnpackedSize},
		chunkHeader{control: copyCtrl,
			unpackedSize: maxCopyUnpackedSize},
		chunkHeader{control: copyResetDictCtrl,
			unpackedSize: minUnpackedSize},
		chunkHeader{control: copyResetDictCtrl,
			unpackedSize: maxCopyUnpackedSize},
		chunkHeader{control: packedCtrl,
			unpackedSize: minUnpackedSize,
			packedSize:   minPackedSize},
		chunkHeader{control: packedCtrl,
			unpackedSize: maxUnpackedSize,
			packedSize:   minPackedSize},
		chunkHeader{control: packedCtrl,
			unpackedSize: maxUnpackedSize,
			packedSize:   maxPackedSize},
		chunkHeader{control: packedResetStateCtrl,
			unpackedSize: minUnpackedSize,
			packedSize:   minPackedSize},
		chunkHeader{control: packedResetStateCtrl,
			unpackedSize: maxUnpackedSize,
			packedSize:   minPackedSize},
		chunkHeader{control: packedResetStateCtrl,
			unpackedSize: maxUnpackedSize,
			packedSize:   maxPackedSize},
		chunkHeader{control: packedNewPropsCtrl,
			unpackedSize: minUnpackedSize,
			packedSize:   minPackedSize,
			props:        props},
		chunkHeader{control: packedNewPropsCtrl,
			unpackedSize: maxUnpackedSize,
			packedSize:   minPackedSize,
			props:        props},
		chunkHeader{control: packedNewPropsCtrl,
			unpackedSize: maxUnpackedSize,
			packedSize:   maxPackedSize,
			props:        props},
		chunkHeader{control: packedResetDictCtrl,
			unpackedSize: minUnpackedSize,
			packedSize:   minPackedSize,
			props:        props},
		chunkHeader{control: packedResetDictCtrl,
			unpackedSize: maxUnpackedSize,
			packedSize:   minPackedSize,
			props:        props},
		chunkHeader{control: packedResetDictCtrl,
			unpackedSize: maxUnpackedSize,
			packedSize:   maxPackedSize,
			props:        props},
	}
	for _, h := range tests {
		var (
			buf bytes.Buffer
			err error
		)
		n, err := writeChunkHeader(&buf, h)
		if err != nil {
			t.Errorf("writeChunkHeader(&buf, %v) error %s", h, err)
			continue
		}
		p := make([]byte, n)
		copy(p, buf.Bytes())
		hr, err := readChunkHeader(&buf)
		if err != nil {
			t.Errorf("readChunkHeader error %s", err)
			continue
		}
		buf.Reset()
		ctrl := hr.control.pure()
		if ctrl != h.control {
			t.Errorf("ctrl %#02x; want %#02x", ctrl, h.control)
			continue
		}
		if _, err = writeChunkHeader(&buf, hr); err != nil {
			t.Errorf("writeChunkHeader(&buf, %v) error %s", hr, err)
			continue
		}
		if !bytes.Equal(p, buf.Bytes()) {
			t.Errorf("buffer %v; want %v", buf.Bytes(), p)
		}
	}
}
