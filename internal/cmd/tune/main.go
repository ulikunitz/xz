package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/ulikunitz/lz"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

// config wraps [xz.WriterConfig] to add our own methods.
type config struct {
	xz.WriterConfig
}

func (c config) sequencerName() string {
	s := c.String()
	x, _, ok := strings.Cut(c.String(), "-")
	if !ok {
		return s
	}
	return x
}

func (c config) String() string {
	switch x := c.LZMA.LZ.(type) {
	case *lz.HSConfig:
		return fmt.Sprintf("HS-%d:%d/%d",
			x.InputLen, x.HashBits, x.WindowSize)
	case *lz.BHSConfig:
		return fmt.Sprintf("BHS-%d:%d/%d",
			x.InputLen, x.HashBits, x.WindowSize)
	case *lz.DHSConfig:
		return fmt.Sprintf("DHS-%d:%d-%d:%d/%d",
			x.InputLen1, x.HashBits1,
			x.InputLen2, x.HashBits2,
			x.WindowSize)
	case *lz.BDHSConfig:
		return fmt.Sprintf("BDHS-%d:%d-%d:%d/%d",
			x.InputLen1, x.HashBits1,
			x.InputLen2, x.HashBits2,
			x.WindowSize)
	case *lz.BUHSConfig:
		return fmt.Sprintf("BUHS-%d:%d:%d/%d",
			x.InputLen, x.HashBits, x.BucketSize, x.WindowSize)
	case *lz.GSASConfig:
		return fmt.Sprintf("GSAS-%d/%d",
			x.MinMatchLen, x.WindowSize)
	default:
		return "(unknown sequencer)"
	}
}

func (c *config) memBudget() int64 {
	switch x := c.LZMA.LZ.(type) {
	case *lz.HSConfig:
		return int64(8)<<x.HashBits + int64(x.BufferSize)
	case *lz.BHSConfig:
		return int64(8)<<x.HashBits + int64(x.BufferSize)
	case *lz.DHSConfig:
		return int64(8)<<x.HashBits1 + int64(8)<<x.HashBits2 +
			int64(x.BufferSize)
	case *lz.BDHSConfig:
		return int64(8)<<x.HashBits1 + int64(8)<<x.HashBits2 +
			int64(x.BufferSize)
	case *lz.BUHSConfig:
		return 8*int64(x.BucketSize)<<x.HashBits + int64(x.BufferSize)
	case *lz.GSASConfig:
		panic("do memory budget for GSAS")
	default:
		panic("unknown sequencer")
	}

}

func (c *config) disable() { c.Workers = -1 }

func (c *config) disabled() bool { return c.Workers < 0 }

func (c *config) worse(o *config) bool {
	if c == nil || o == nil || c == o {
		return false
	}
	d, e := c.LZMA.DictSize, o.LZMA.DictSize
	switch x := c.LZMA.LZ.(type) {
	case *lz.HSConfig:
		y, ok := o.LZMA.LZ.(*lz.HSConfig)
		if !(ok && x.InputLen == y.InputLen) {
			return false
		}
		return d <= e && x.HashBits <= y.HashBits
	case *lz.BHSConfig:
		y, ok := o.LZMA.LZ.(*lz.BHSConfig)
		if !(ok && x.InputLen == y.InputLen) {
			return false
		}
		return d <= e && x.HashBits <= y.HashBits
	case *lz.DHSConfig:
		y, ok := o.LZMA.LZ.(*lz.DHSConfig)
		if !ok {
			return false
		}
		if !(x.InputLen1 == y.InputLen1 && x.InputLen2 == y.InputLen2) {
			return false
		}
		return d <= e && x.HashBits1 <= y.HashBits1 && x.HashBits2 <= y.HashBits2
	case *lz.BDHSConfig:
		y, ok := o.LZMA.LZ.(*lz.BDHSConfig)
		if !ok {
			return false
		}
		if !(x.InputLen1 == y.InputLen1 && x.InputLen2 == y.InputLen2) {
			return false
		}
		return d <= e && x.HashBits1 <= y.HashBits1 && x.HashBits2 <= y.HashBits2
	case *lz.BUHSConfig:
		y, ok := o.LZMA.LZ.(*lz.BUHSConfig)
		if !(ok && x.InputLen == y.InputLen) {
			return false
		}
		return d <= e && x.HashBits <= y.HashBits && x.BucketSize <= y.BucketSize
	default:
		return false
	}
}

// mbPerSec returns the Megabytes (1 000 000 bytes) per seconds that are
// processed.
func mbPerSec(r testing.BenchmarkResult) float64 {
	if v, ok := r.Extra["MB/s"]; ok {
		return v
	}
	if r.Bytes <= 0 || r.T <= 0 || r.N <= 0 {
		return 0
	}
	return (float64(r.Bytes) * float64(r.N) / 1e6) / r.T.Seconds()
}

func ratio(r testing.BenchmarkResult) float64 {
	if x, ok := r.Extra["c/u"]; ok {
		return x
	}
	return math.NaN()
}

type info struct {
	c config
	r testing.BenchmarkResult
}

func bestConfigsForSlot(nr int, configs []config, minMemSize, maxMemSize int64) (infos []info) {

	fmt.Printf("\n## Start Slot %d\n\n", nr)
	var slotConfigs []config
	for _, c := range configs {
		m := c.memBudget()
		if !(minMemSize < m && m <= maxMemSize) {
			continue
		}
		slotConfigs = append(slotConfigs, c)
	}
	m := make(map[string]info, 5)

	rand.Shuffle(len(slotConfigs), func(i, j int) {
		slotConfigs[i], slotConfigs[j] = slotConfigs[j], slotConfigs[i]
	})

	i := 0
	todo := len(slotConfigs)
	for len(slotConfigs) > 0 {
		n := len(slotConfigs)
		cfg := slotConfigs[n-1]
		slotConfigs = slotConfigs[:n-1]
		if cfg.disabled() {
			continue
		}
		todo--
		i++

		r := testing.Benchmark(writerBenchmark(cfg.WriterConfig))
		var added = ""
		name := cfg.sequencerName()
		f, ok := m[name]
		if !ok || ratio(r) < ratio(f.r) {
			added = "+++"
			m[name] = info{cfg, r}
		}
		for k, c := range slotConfigs {
			if c.disabled() {
				continue
			}
			if c.worse(&cfg) {
				todo--
				slotConfigs[k].disable()
			}
		}
		fmt.Printf("%d/%d\t%s\t%.2f MB/s %.3f c/u %s\n", i, todo, cfg,
			mbPerSec(r), ratio(r), added)
	}

	infos = make([]info, 0, len(m))
	for _, f := range m {
		infos = append(infos, f)
	}

	sort.Slice(infos, func(i, j int) bool {
		return ratio(infos[i].r) < ratio(infos[j].r)
	})

	fmt.Printf("\n## Slot %d Results\n\n", nr)

	for j, f := range infos {
		fmt.Printf("%d.\t%s\t%.3f c/u %.2f MB/s\n",
			j+1, f.c, ratio(f.r), mbPerSec(f.r))
	}

	return infos
}

func makeConfig(cfg lz.SeqConfig, windowSize int) config {
	wc := xz.WriterConfig{
		LZMA: lzma.Writer2Config{
			Workers:  1,
			DictSize: windowSize,
			LZ:       cfg,
		},
		Workers: 1,
	}
	return config{wc}
}
func appendHSConfigs(x []config) (y []config) {
	y = x
	for windowExp := 15; windowExp <= 26; windowExp++ {
		for hashBits := 4; hashBits <= 23; hashBits++ {
			for _, inputLen := range []int{3, 4} {
				cfg := makeConfig(
					&lz.HSConfig{
						InputLen: inputLen,
						HashBits: hashBits,
					},
					1<<windowExp,
				)
				cfg.SetDefaults()
				y = append(y, cfg)
			}
		}
	}
	return y
}

func appendBHSConfigs(x []config) (y []config) {
	y = x
	for windowExp := 15; windowExp <= 26; windowExp++ {
		for hashBits := 4; hashBits <= 23; hashBits++ {
			for _, inputLen := range []int{3, 4} {
				cfg := makeConfig(
					&lz.BHSConfig{
						InputLen: inputLen,
						HashBits: hashBits,
					},
					1<<windowExp,
				)
				cfg.SetDefaults()
				y = append(y, cfg)
			}
		}
	}
	return y
}

func appendDHSConfigs(x []config) (y []config) {
	y = x
	for wexp := 15; wexp <= 26; wexp++ {
		for hb1 := 4; hb1 <= 23; hb1++ {
			for _, il1 := range []int{3, 4} {
				for il2 := il1 + 1; il2 <= 8; il2++ {
					for hb2 := hb1; hb2 <= 23; hb2++ {
						cfg := makeConfig(
							&lz.DHSConfig{
								InputLen1: il1,
								InputLen2: il2,
								HashBits1: hb1,
								HashBits2: hb2,
							},
							1<<wexp,
						)
						cfg.SetDefaults()
						y = append(y, cfg)

					}
				}
			}
		}
	}
	return y
}

func appendBDHSConfigs(x []config) (y []config) {
	y = x
	for wexp := 15; wexp <= 26; wexp++ {
		for hb1 := 4; hb1 <= 23; hb1++ {
			for _, il1 := range []int{3, 4} {
				for il2 := il1 + 1; il2 <= 8; il2++ {
					for hb2 := hb1; hb2 <= 23; hb2++ {
						cfg := makeConfig(
							&lz.BDHSConfig{
								InputLen1: il1,
								InputLen2: il2,
								HashBits1: hb1,
								HashBits2: hb2,
							},
							1<<wexp,
						)
						cfg.SetDefaults()
						y = append(y, cfg)

					}
				}
			}
		}
	}
	return y
}

func appendBUHSConfigs(x []config) (y []config) {
	y = x
	for windowExp := 15; windowExp <= 26; windowExp++ {
		for hashBits := 4; hashBits <= 23; hashBits++ {
			for bucketSize := 4; bucketSize <= 30; bucketSize++ {
				cfg := makeConfig(
					&lz.BUHSConfig{
						InputLen:   3,
						HashBits:   hashBits,
						BucketSize: bucketSize,
					},
					1<<windowExp,
				)
				cfg.SetDefaults()
				y = append(y, cfg)
			}
		}
	}
	return y
}

func main() {
	testing.Init()
	configs := appendHSConfigs(nil)
	configs = appendBHSConfigs(configs)
	configs = appendDHSConfigs(configs)
	configs = appendBDHSConfigs(configs)
	configs = appendBUHSConfigs(configs)

	slots := []int64{1 << 20, 1 << 21, 1 << 22, 1 << 23, 1 << 24, 1 << 25,
		1 << 26, 1 << 27, 1 << 28}
	var infoSlots [][]info
	mOld := int64(0)
	for i, m := range slots {
		infos := bestConfigsForSlot(i+1, configs, mOld, m)
		mOld = m
		infoSlots = append(infoSlots, infos)
	}

	_ = infoSlots
}
