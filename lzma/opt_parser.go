package lzma

import (
	"fmt"
	"reflect"

	"github.com/ulikunitz/lz"
)

// This file will include an optimized parser that relies on lzma encoder to
// compute the costs for the matches and literals.

// prime is used by [hashValue]
const prime = 9920624304325388887

// hashValue computes a hash from the string stored in x with the first byte
// stored on the lowest bits. The shift values ensures that only 64 - shift bits
// potential non-zero bits remain.
func hashValue(x uint64, shift uint) uint32 {
	return uint32((x * prime) >> shift)
}

type bucketEntry struct {
	pos uint32
	val uint32
}

type bucket []bucketEntry

type bucketHash struct {
	table    []bucket
	mask     uint64
	shift    uint
	inputLen int
}

func (h *bucketHash) init(inputLen, hashBits int) error {
	if !(2 <= inputLen && inputLen <= 8) {
		return fmt.Errorf("lz: inputLen must be in the range [2,8]")
	}
	maxHashBits := 24
	if t := 8 * inputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= hashBits && hashBits <= maxHashBits) {
		return fmt.Errorf("lz: hashBits=%d; must be <= %d",
			hashBits, maxHashBits)
	}

	n := 1 << hashBits
	if n <= cap(h.table) {
		h.table = h.table[:n]
		for i := range h.table {
			h.table[i] = nil
		}
	} else {
		h.table = make([]bucket, n)
	}

	h.mask = 1<<(uint(inputLen)*8) - 1
	h.shift = 64 - uint(hashBits)
	h.inputLen = inputLen

	return nil
}

func (h *bucketHash) reset() {
	for i := range h.table {
		h.table[i] = nil
	}
}

func (bh *bucketHash) add(h, pos, val uint32) {
	b := &bh.table[h]
	*b = append(*b, bucketEntry{pos: pos, val: val})
}

func (bh *bucketHash) shiftOffsets(delta uint32) {
	if delta == 0 {
		return
	}

	for h, b := range bh.table {
		i := -1
		for j, e := range b {
			if e.pos < delta {
				continue
			}
			if i < 0 {
				i = j
			}
			b[j].pos -= delta
		}
		k := 0
		if i >= 0 {
			k = copy(b, b[i:])
		}
		bh.table[h] = b[:k]
	}
}

type bucketConfig struct {
	InputLen int
	HashBits int
}

func hasVal(v reflect.Value, name string) bool {
	_, ok := v.Type().FieldByName(name)
	return ok
}

func iVal(v reflect.Value, name string) int {
	return int(v.FieldByName(name).Int())
}

func setIVal(v reflect.Value, name string, i int) {
	v.FieldByName(name).SetInt(int64(i))
}

var errNoBucketConfig = fmt.Errorf("lz,a: no bucket config found")

func bucketCfg(cfg lz.ParserConfig) (bcfg bucketConfig, err error) {
	v := reflect.Indirect(reflect.ValueOf(cfg))
	if !(hasVal(v, "InputLen") && hasVal(v, "HashBits")) {
		return bcfg, errNoBucketConfig
	}
	bcfg = bucketConfig{
		InputLen: iVal(v, "InputLen"),
		HashBits: iVal(v, "HashBits"),
	}
	return bcfg, nil
}

func setBucketCfg(cfg lz.ParserConfig, bcfg bucketConfig) error {
	v := reflect.Indirect(reflect.ValueOf(cfg))
	if !(hasVal(v, "InputLen") && hasVal(v, "HashBits")) {
		return errNoBucketConfig
	}
	setIVal(v, "InputLen", bcfg.InputLen)
	setIVal(v, "HashBits", bcfg.HashBits)
	return nil
}

func (cfg *bucketConfig) SetDefaults() {
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 18
	}
}

func (cfg *bucketConfig) Validate() error {
	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf("lz: InputLen must be in the range [2,8]")
	}
	maxHashBits := 24
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be <= %d",
			cfg.HashBits, maxHashBits)
	}
	return nil
}

type bucketDict struct {
	lz.Buffer
	bucketHash
}

func (d *bucketDict) init(cfg bucketConfig, bcfg lz.BufConfig) error {
	var err error
	if err = d.Buffer.Init(bcfg); err != nil {
		return err
	}
	if err = d.bucketHash.init(cfg.InputLen, cfg.HashBits); err != nil {
		return err
	}
	return nil
}

func (d *bucketDict) Reset(data []byte) error {
	var err error
	if err = d.Buffer.Reset(data); err != nil {
		return err
	}
	d.bucketHash.reset()
	return nil
}

func (d *bucketDict) Shrink() int {
	delta := d.Buffer.Shrink()
	d.bucketHash.shiftOffsets(uint32(delta))
	return delta
}

func (d *bucketDict) processSegment(a, b int) {
	if a < 0 {
		a = 0
	}
	c := len(d.Data) - d.inputLen + 1
	if b > c {
		b = c
	}
	if b <= 0 {
		return
	}

	_p := d.Data[:b+7]
	for i := a; i < b; i++ {
		x := _getLE64(_p[i:]) & d.mask
		h := hashValue(x, d.shift)
		d.add(h, uint32(i), uint32(x))
	}
}