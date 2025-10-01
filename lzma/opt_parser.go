package lzma

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"slices"

	"github.com/ulikunitz/lz"
)

var logger *slog.Logger

func init() {
	f, err := os.CreateTemp(".", "op_*.out")
	if err != nil {
		panic(err)
	}
	h := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger = slog.New(h)
}

// This file will include an optimized parser that relies on lzma encoder to
// compute the costs for the matches and literals.

type optParser struct {
	chtDict

	OPConfig

	updateEncoder func() *encoder

	i        int
	optTable []optItem
}

const maxCost = (1 << 64) - 1

type optItem struct {
	cost    uint64
	counter *counter
	// if offset == 0, contains a literal byte
	len    uint32
	offset uint32
}

func (s *optParser) initOptTable() {
	n := s.BlockSize + 1
	if cap(s.optTable) >= n {
		s.optTable = s.optTable[:n]
	} else {
		s.optTable = make([]optItem, n)
	}
	for i := range s.optTable {
		s.optTable[i] = optItem{cost: maxCost}
	}
	ctr := new(counter)
	ctr.fromEncoder(s.updateEncoder())
	item0 := &s.optTable[0]
	item0.cost = 0
	item0.counter = ctr
}

func (s *optParser) addLiteral(b uint32) error {
	b &= 0xff
	k := s.i - s.W
	itemA := &s.optTable[k]
	ctr := new(counter)
	ctr.copy(itemA.counter)
	n := ctr.bits()
	if err := ctr.writeLiteral(byte(b)); err != nil {
		return err
	}
	n = ctr.bits() - n
	itemB := &s.optTable[k+1]
	cost := itemA.cost + uint64(n)
	if cost >= itemB.cost {
		return nil
	}
	itemB.cost = cost
	itemB.len = b
	itemB.offset = 0
	itemB.counter = ctr
	return nil
}

func (s *optParser) addMatch(dist, matchLen uint32) (updated bool, err error) {
	if !(minMatchLen <= matchLen && matchLen <= maxMatchLen) {
		return false, fmt.Errorf("lzma: match length %d out of range", matchLen)
	}
	if !(0 < int(dist) && int(dist) <= s.WindowSize) {
		return false, fmt.Errorf("lzma: match distance %d out of range", dist)
	}
	k := s.i - s.W
	kB := k + int(matchLen)
	if kB >= len(s.optTable) {
		return false, fmt.Errorf(
			"lzma: match at i=%d with length %d exceeds the block size",
			s.i, matchLen)
	}
	itemA, itemB := &s.optTable[k], &s.optTable[kB]
	ctr := new(counter)
	ctr.copy(itemA.counter)
	n := ctr.bits()
	if err := ctr.writeMatch(dist, matchLen); err != nil {
		return false, err
	}
	cost := itemA.cost + uint64(ctr.bits()-n)
	if cost >= itemB.cost {
		return false, nil
	}
	itemB.cost = cost
	itemB.len = matchLen
	itemB.offset = dist
	itemB.counter = ctr
	return true, nil
}

func (s *optParser) fillBlock(blk *lz.Block, flags int) (n int, err error) {
	if blk == nil {
		return 0, fmt.Errorf("lzma: internal error; blk == nil")
	}
	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	var seq lz.Seq

	k := s.i - s.W

	if flags&lz.NoTrailingLiterals != 0 {
		for k > 0 && s.optTable[k].offset == 0 {
			k--
		}
	}

	n = k
	for k > 0 {
		item := &s.optTable[k]
		if item.offset == 0 {
			blk.Literals = append(blk.Literals, byte(item.len))
			seq.LitLen++
			k--
			continue
		}
		if seq.MatchLen != 0 {
			blk.Sequences = append(blk.Sequences, seq)
		}
		if item.len < minMatchLen {
			return 0, fmt.Errorf(
				"lzma: internal error; match length %d < minMatchLen %d",
				item.len, minMatchLen)
		}
		seq = lz.Seq{
			Offset:   item.offset,
			MatchLen: item.len,
		}
		k -= int(item.len)
	}
	if k != 0 {
		return 0, fmt.Errorf("lzma: internal error; k=%d", k)
	}
	if seq.MatchLen != 0 {
		blk.Sequences = append(blk.Sequences, seq)
	}

	slices.Reverse(blk.Sequences)
	slices.Reverse(blk.Literals)
	// move the counters to the garbage collector
	for i := range s.optTable {
		s.optTable[i] = optItem{cost: maxCost}
	}
	return n, nil
}

func setUpdateEncoder(p lz.Parser, updateEncoder func() *encoder) {
	op, ok := p.(*optParser)
	if !ok {
		return
	}
	op.updateEncoder = updateEncoder
}

type OPConfig struct {
	InputLen int
	HashBits int

	ShrinkSize int
	BufferSize int
	WindowSize int
	BlockSize  int
}

func (cfg *OPConfig) Clone() lz.ParserConfig {
	x := *cfg
	return &x
}

func (cfg *OPConfig) UnmarshalJSON(p []byte) error {
	*cfg = OPConfig{}
	return lz.UnmarshalJSON(cfg, p)
}

func (cfg *OPConfig) MarshalJSON() (p []byte, err error) {
	return lz.MarshalJSON(cfg)
}

func (cfg *OPConfig) BufConfig() lz.BufferConfig {
	bc := lz.BufConfig(cfg)
	return bc
}

func (cfg *OPConfig) SetBufConfig(bc lz.BufferConfig) {
	lz.SetBufConfig(cfg, bc)
}

func (cfg *OPConfig) SetDefaults() {
	bc := lz.BufConfig(cfg)
	bc.SetDefaults()
	lz.SetBufConfig(cfg, bc)
	chtCfg, _ := chainingHashTableCfg(cfg)
	chtCfg.SetDefaults()
	setChainingHashTableCfg(cfg, chtCfg)
}

func (cfg *OPConfig) Verify() error {
	bc := lz.BufConfig(cfg)
	var err error
	if err = bc.Verify(); err != nil {
		return err
	}
	chtCfg, _ := chainingHashTableCfg(cfg)
	err = chtCfg.Verify()
	return err
}

func (cfg OPConfig) NewParser() (p lz.Parser, err error) {
	op := new(optParser)
	if err = op.init(cfg); err != nil {
		return nil, err
	}
	return op, nil
}

func (s *optParser) init(cfg OPConfig) error {
	cfg.SetDefaults()
	if err := cfg.Verify(); err != nil {
		return err
	}

	chtCfg, _ := chainingHashTableCfg(&cfg)
	bc := lz.BufConfig(&cfg)
	if err := s.chtDict.init(chtCfg, bc); err != nil {
		return err
	}

	s.OPConfig = cfg

	s.optTable = make([]optItem, cfg.BlockSize+1)

	return nil
}

func (s *optParser) ParserConfig() lz.ParserConfig {
	return &s.OPConfig
}

func (s *optParser) Parse(blk *lz.Block, flags int) (n int, err error) {
	n = min(len(s.Data)-s.W, s.BlockSize)

	if blk == nil {
		if n == 0 {
			return 0, lz.ErrEmptyBuffer
		}
		t := s.W + n
		s.processSegment(s.W-s.inputLen+1, t)
		s.W = t
		return n, nil
	}

	if n == 0 {
		return 0, lz.ErrEmptyBuffer
	}

	s.processSegment(s.W-s.inputLen+1, s.W)
	p := s.Data[:s.W+n]

	inputEnd := len(p) - s.inputLen + 1
	s.i = s.W

	minMatchLen := min(s.inputLen, 2)

	_p := s.Data[:inputEnd+7]

	s.initOptTable()

	for ; s.i < inputEnd; s.i++ {
		x := _getLE64(_p[s.i:]) & s.mask
		h := hashValue(x, s.shift)
		v := uint32(x)
		if err = s.addLiteral(v); err != nil {
			return 0, err
		}
		for _, e := range s.table[h] {
			if v != e.val {
				continue
			}
			j := int(e.pos)
			oe := s.i - j
			if !(0 < oe && oe <= s.WindowSize) {
				continue
			}
			ke := lcp(p[j:], p[s.i:])
			if ke < minMatchLen {
				continue
			}
			if ke >= maxMatchLen {
				ke = maxMatchLen
			}
			for ; ke >= minMatchLen; ke-- {
				updated, err := s.addMatch(uint32(oe), uint32(ke))
				if err != nil {
					return 0, err
				}

				// TODO remove
				var str string
				if updated {
					str = "add match"
				} else {
					str = "skip match"
				}
				logger.Debug(str,
					slog.Int("i", s.i),
					slog.Int("len", ke),
					slog.Int("dist", oe),
					slog.String("val", fmt.Sprintf("0x%08x", v)),
				)
			}
		}
		s.add(h, uint32(s.i), v)
	}
	for ; s.i < len(p); s.i++ {
		v := uint32(p[s.i])
		if err = s.addLiteral(v); err != nil {
			return 0, err
		}
	}

	n, err = s.fillBlock(blk, flags)
	if err != nil {
		return 0, err
	}
	s.W += n
	return n, nil
}

// prime is used by [hashValue]
const prime = 9920624304325388887

// hashValue computes a hash from the string stored in x with the first byte
// stored on the lowest bits. The shift values ensures that only 64 - shift bits
// potential non-zero bits remain.
func hashValue(x uint64, shift uint) uint32 {
	return uint32((x * prime) >> shift)
}

type chainEntry struct {
	pos uint32
	val uint32
}

type chain []chainEntry

type chainingHashTable struct {
	table    []chain
	mask     uint64
	shift    uint
	inputLen int
}

func (ch *chainingHashTable) init(inputLen, hashBits int) error {
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
	if n <= cap(ch.table) {
		ch.table = ch.table[:n]
		for i := range ch.table {
			ch.table[i] = nil
		}
	} else {
		ch.table = make([]chain, n)
	}

	ch.mask = 1<<(uint(inputLen)*8) - 1
	ch.shift = 64 - uint(hashBits)
	ch.inputLen = inputLen

	return nil
}

func (ch *chainingHashTable) reset() {
	for i := range ch.table {
		ch.table[i] = nil
	}
}

func (ch *chainingHashTable) add(h, pos, val uint32) {
	b := &ch.table[h]
	*b = append(*b, chainEntry{pos: pos, val: val})
}

func (ch *chainingHashTable) shiftOffsets(delta uint32) {
	if delta == 0 {
		return
	}

	for h, b := range ch.table {
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
		ch.table[h] = b[:k]
	}
}

type chainingHashTableConfig struct {
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

var errNoBucketConfig = fmt.Errorf("lzma: no bucket config found")

func chainingHashTableCfg(cfg lz.ParserConfig) (chtCfg chainingHashTableConfig, err error) {
	v := reflect.Indirect(reflect.ValueOf(cfg))
	if !(hasVal(v, "InputLen") && hasVal(v, "HashBits")) {
		return chtCfg, errNoBucketConfig
	}
	chtCfg = chainingHashTableConfig{
		InputLen: iVal(v, "InputLen"),
		HashBits: iVal(v, "HashBits"),
	}
	return chtCfg, nil
}

func setChainingHashTableCfg(cfg lz.ParserConfig, chtCfg chainingHashTableConfig) error {
	v := reflect.Indirect(reflect.ValueOf(cfg))
	if !(hasVal(v, "InputLen") && hasVal(v, "HashBits")) {
		return errNoBucketConfig
	}
	setIVal(v, "InputLen", chtCfg.InputLen)
	setIVal(v, "HashBits", chtCfg.HashBits)
	return nil
}

func (cfg *chainingHashTableConfig) SetDefaults() {
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 18
	}
}

func (cfg *chainingHashTableConfig) Verify() error {
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

type chtDict struct {
	lz.Buffer
	chainingHashTable
}

func (d *chtDict) init(cfg chainingHashTableConfig, bcfg lz.BufferConfig) error {
	var err error
	if err = d.Buffer.Init(bcfg); err != nil {
		return err
	}
	if err = d.chainingHashTable.init(cfg.InputLen, cfg.HashBits); err != nil {
		return err
	}
	return nil
}

func (d *chtDict) Reset(data []byte) error {
	var err error
	if err = d.Buffer.Reset(data); err != nil {
		return err
	}
	d.chainingHashTable.reset()
	return nil
}

func (d *chtDict) Shrink() int {
	delta := d.Buffer.Shrink()
	d.chainingHashTable.shiftOffsets(uint32(delta))
	return delta
}

func (d *chtDict) processSegment(a, b int) {
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
