package main

import (
	"fmt"
	"log"
	"sort"
	"testing"

	"github.com/kr/pretty"
	"github.com/ulikunitz/lz"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

type preset struct {
	cfg    xz.WriterConfig
	result testing.BenchmarkResult
}

func findPresets(slots []float64, configs []xz.WriterConfig) {
	if len(slots) == 0 {
		log.Fatalf("no slots defined")
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i] > slots[j]
	})
	fmt.Printf("slots %.3f\n", slots)

	for i, cfg := range configs {
		result := testing.Benchmark(writerBenchmark(cfg))
		pretty.Println(cfg)
		fmt.Printf("%d/%d %s\n", i+1, len(configs), result)
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
func appendHSConfigs(c []xz.WriterConfig) (r []xz.WriterConfig) {
	r = c
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
				r = append(r, cfg)
			}
		}
	}
	return r
}

func main() {
	testing.Init()
	configs := appendHSConfigs(nil)

	slots := []float64{0.28, 0.27, 0.26, 0.25,
		0.24, 0.23, 0.225, 0.22, 0.215}
	findPresets(slots, configs)
}
