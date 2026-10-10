package nlzma

import (
	"fmt"
	"testing"
)

func TestRangeCodec(t *testing.T) {
	tests := []string{
		"",
		"abc",
		"1234567890",
		"the quick brown fox jumps over the lazy dog",
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("t=%d", i), func(t *testing.T) {
			data := []byte(tc)
			n := len(data)

			var e rEncoder
			e.init(make([]byte, 0, 32))

			var pr [4]prob
			for i := range pr {
				pr[i] = probInit
			}
			for _, _b := range data {
				b := uint32(_b)
				e.directEncodeBit(b)
				pr[0] = e.encodeBit(pr[0], b>>1)
				e.directEncodeBit(b >> 2)
				pr[1] = e.encodeBit(pr[1], b>>3)
				e.directEncodeBit(b >> 4)
				pr[2] = e.encodeBit(pr[2], b>>5)
				e.directEncodeBit(b >> 6)
				pr[3] = e.encodeBit(pr[3], b>>7)
			}
			if err := e.Close(); err != nil {
				t.Fatalf("encoder close failed: %v", err)
			}

			out := make([]byte, 0, n)
			var d rDecoder
			d.init(e.p)

			var prDec [4]prob
			for i := range prDec {
				prDec[i] = probInit
			}
			for range n {
				var c byte
				b := d.directDecodeBit()
				c |= byte(b)
				b, prDec[0] = d.decodeBit(prDec[0])
				c |= byte(b << 1)
				b = d.directDecodeBit()
				c |= byte(b << 2)
				b, prDec[1] = d.decodeBit(prDec[1])
				c |= byte(b << 3)
				b = d.directDecodeBit()
				c |= byte(b << 4)
				b, prDec[2] = d.decodeBit(prDec[2])
				c |= byte(b << 5)
				b = d.directDecodeBit()
				c |= byte(b << 6)
				b, prDec[3] = d.decodeBit(prDec[3])
				c |= byte(b << 7)
				out = append(out, c)
				if len(d.p) == 0 {
					break
				}
			}
			if !d.possiblyAtEnd() {
				t.Fatalf("decoder not at end")
			}
			if string(out) != tc {
				t.Fatalf("decoded output mismatch: got %q, want %q", string(out), tc)
			}
		})
	}
}
