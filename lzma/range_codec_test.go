package lzma

import (
	"bytes"
	"testing"
)

func floatProb(p prob) float64 {
	return float64(p) / float64(1<<probBits)
}

func TestIncProb(t *testing.T) {
	p := probInit
	for {
		t.Logf("p %.10f. %b", floatProb(p), p)
		q := incProb(p)
		if q == p {
			break
		}
		p = q
	}
}

func TestDecProb(t *testing.T) {
	p := probInit
	for {
		t.Logf("p %.10f. %011b", floatProb(p), p)
		q := decProb(p)
		if q == p {
			break
		}
		p = q
	}
}

func encodeByte(e rEncoder, b uint8, p []prob) error {
	x := uint32(b)
	e.directEncodeBit(x)
	var err error
	if err = e.encodeBit(x>>1, &p[0]); err != nil {
		return err
	}
	if err = e.encodeBit(x>>2, &p[1]); err != nil {
		return err
	}
	if err = e.encodeBit(x>>3, &p[2]); err != nil {
		return err
	}
	if err = e.encodeBit(x>>4, &p[3]); err != nil {
		return err
	}
	if err = e.encodeBit(x>>5, &p[4]); err != nil {
		return err
	}
	if err = e.encodeBit(x>>6, &p[5]); err != nil {
		return err
	}
	if err = e.encodeBit(x>>7, &p[6]); err != nil {
		return err
	}
	return nil
}

func decodeByte(d *rangeDecoder, p []prob) (uint8, error) {
	x := uint32(0)
	var err error
	if x, err = d.directDecodeBit(); err != nil {
		return 0, err
	}
	var b uint32
	if b, err = d.decodeBit(&p[0]); err != nil {
		return 0, err
	}
	x |= b << 1
	if b, err = d.decodeBit(&p[1]); err != nil {
		return 0, err
	}
	x |= b << 2
	if b, err = d.decodeBit(&p[2]); err != nil {
		return 0, err
	}
	x |= b << 3
	if b, err = d.decodeBit(&p[3]); err != nil {
		return 0, err
	}
	x |= b << 4
	if b, err = d.decodeBit(&p[4]); err != nil {
		return 0, err
	}
	x |= b << 5
	if b, err = d.decodeBit(&p[5]); err != nil {
		return 0, err
	}
	x |= b << 6
	if b, err = d.decodeBit(&p[6]); err != nil {
		return 0, err
	}
	x |= b << 7
	return uint8(x), nil

}

func TestRangeBitCounter(t *testing.T) {
	const text = "The brown fox jumps over the lazy dog."
	probs := make([]prob, 7)
	for i := range probs {
		probs[i] = probInit
	}

	data := []byte(text)
	buf := new(bytes.Buffer)
	var e rangeEncoder
	e.init(buf)

	for _, b := range data {
		if err := encodeByte(&e, b, probs); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("encoded %d bytes into %d bytes", len(data), buf.Len())

	var d rangeDecoder
	d.init(buf)
	for i := range probs {
		probs[i] = probInit
	}

	for _, a := range data {
		b, err := decodeByte(&d, probs)
		if err != nil {
			t.Fatal(err)
		}
		if b != a {
			t.Errorf(
				"decoded byte %d does not match original byte %d",
				b, a)
		}
	}
	if !d.possiblyAtEnd() {
		t.Error("decoder is not at end")
	}

	for i := range probs {
		probs[i] = probInit
	}
	var c rangeBitCounter
	c.init()
	start := c.bits()
	t.Logf("range bit counter start: %d", start)
	for _, b := range data {
		if err := encodeByte(&c, b, probs); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("bits before Close: %d", c.bits()-start)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	n := c.bits() - start
	t.Logf("range bit counter: %d bits (%d bytes)", n, (n+7)/8)
}
