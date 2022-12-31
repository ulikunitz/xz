package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/kr/pretty"
	"github.com/ulikunitz/lz"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

type preset struct {
	present bool
	cfg     xz.WriterConfig
	result  testing.BenchmarkResult
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

// Returns the slot index the ratio qualifies for. If no slot can be found ok
// will be false.
func slot(slots []float64, ratio float64) (i int, ok bool) {
	for i, r := range slots {
		if ratio > r {
			return i - 1, i > 0
		}
	}
	return len(slots) - 1, true
}

func disable(cfg *xz.WriterConfig) { cfg.Workers = -1 }

func disabled(cfg *xz.WriterConfig) bool { return cfg.Workers < 0 }

func worse(a, b *xz.WriterConfig) bool {
	if a == nil || b == nil || a == b {
		return false
	}
	d, e := a.LZMA.DictSize, b.LZMA.DictSize
	switch x := a.LZMA.LZ.(type) {
	case *lz.HSConfig:
		y, ok := b.LZMA.LZ.(*lz.HSConfig)
		if !(ok && x.InputLen == y.InputLen) {
			return false
		}
		return d <= e && x.HashBits <= y.HashBits
	case *lz.BHSConfig:
		y, ok := b.LZMA.LZ.(*lz.BHSConfig)
		if !(ok && x.InputLen == y.InputLen) {
			return false
		}
		return d <= e && x.HashBits <= y.HashBits
	case *lz.DHSConfig:
		y, ok := b.LZMA.LZ.(*lz.DHSConfig)
		if !ok {
			return false
		}
		if !(x.InputLen1 == y.InputLen1 && x.InputLen2 == y.InputLen2) {
			return false
		}
		return d <= e && x.HashBits1 <= y.HashBits1 && x.HashBits2 <= y.HashBits2
	case *lz.BDHSConfig:
		y, ok := b.LZMA.LZ.(*lz.BDHSConfig)
		if !ok {
			return false
		}
		if !(x.InputLen1 == y.InputLen1 && x.InputLen2 == y.InputLen2) {
			return false
		}
		return d <= e && x.HashBits1 <= y.HashBits1 && x.HashBits2 <= y.HashBits2
	case *lz.BUHSConfig:
		y, ok := b.LZMA.LZ.(*lz.BUHSConfig)
		if !(ok && x.InputLen == y.InputLen) {
			return false
		}
		return d <= e && x.HashBits <= y.HashBits && x.BucketSize <= y.BucketSize
	default:
		return false
	}
}

func findPresets(slots []float64, configs []xz.WriterConfig) {
	if len(slots) == 0 {
		log.Fatalf("no slots defined")
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i] > slots[j]
	})
	fmt.Printf("slots %.3f\n", slots)
	rand.Shuffle(len(configs), func(i, j int) {
		configs[i], configs[j] = configs[j], configs[i]
	})

	presets := make([]preset, len(slots))

	i := 0
	n := len(configs)
	for len(configs) > 0 {
		k := len(configs) - 1
		cfg := configs[k]
		configs = configs[:k]
		if disabled(&cfg) {
			continue
		}
		n--

		i++
		result := testing.Benchmark(writerBenchmark(cfg))
		fmt.Printf("%d-%d %s\n", i, n, result)
		si, ok := slot(slots, ratio(result))
		if !ok {
			for i := range configs {
				p := &configs[i]
				if disabled(p) {
					continue
				}
				if worse(p, &cfg) {
					disable(p)
					n--
				}
			}
			continue
		}
		v := mbPerSec(result)
		p := presets[si]
		if p.present && v <= mbPerSec(p.result) {
			fmt.Printf("slot %d - not faster\n", si+1)
			continue
		}
		presets[si] = preset{
			present: true,
			cfg:     cfg,
			result:  result,
		}
		fmt.Printf("slot %d - update\n", si+1)
		pretty.Println(cfg)
	}

	fmt.Printf("\n\n### Result ###\n\n")

	for si, p := range presets {
		if si > 0 {
			fmt.Printf("\n")
		}
		if !p.present {
			fmt.Printf("slot %d - not present\n", si)
			continue
		}
		fmt.Printf("slot %d - \t%.3f c/u\t%.2f MB/s\n",
			si+1, ratio(p.result), mbPerSec(p.result))
		pretty.Println(p.cfg)
	}
}

func makeWriterConfig(cfg lz.SeqConfig, windowSize int) xz.WriterConfig {
	return xz.WriterConfig{
		LZMA: lzma.Writer2Config{
			Workers:  1,
			DictSize: windowSize,
			LZ:       cfg,
		},
		Workers: 1,
	}
}
func appendHSConfigs(x []xz.WriterConfig) (y []xz.WriterConfig) {
	y = x
	for windowExp := 15; windowExp <= 23; windowExp++ {
		for hashBits := 4; hashBits <= 23; hashBits++ {
			for _, inputLen := range []int{3, 4} {
				cfg := makeWriterConfig(
					&lz.HSConfig{
						InputLen: inputLen,
						HashBits: hashBits,
					},
					1<<windowExp,
				)
				cfg.ApplyDefaults()
				y = append(y, cfg)
			}
		}
	}
	return y
}

func appendBUHSConfigs(x []xz.WriterConfig) (y []xz.WriterConfig) {
	y = x
	for windowExp := 15; windowExp <= 23; windowExp++ {
		for hashBits := 4; hashBits <= 23; hashBits++ {
			for bucketSize := 4; bucketSize <= 30; bucketSize++ {
				cfg := makeWriterConfig(
					&lz.BUHSConfig{
						InputLen:   3,
						HashBits:   hashBits,
						BucketSize: bucketSize,
					},
					1<<windowExp,
				)
				cfg.ApplyDefaults()
				y = append(y, cfg)
			}
		}
	}
	return y
}

func main() {
	testing.Init()
	configs := appendHSConfigs(nil)
	configs = appendBUHSConfigs(configs)

	slots := []float64{0.28, 0.27, 0.26, 0.25,
		0.24, 0.23, 0.22, 0.21, 0.20}
	findPresets(slots, configs)
}
