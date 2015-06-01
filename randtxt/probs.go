package randtxt

import (
	"math"
	"math/rand"
	"sort"
)

type ngram struct {
	s   string
	lgP float64
	lgQ float64
}

type ngrams []ngram

func (s ngrams) Len() int           { return len(s) }
func (s ngrams) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s ngrams) Less(i, j int) bool { return s[i].s < s[j].s }

func (s ngrams) Sort() { sort.Sort(s) }

func (s ngrams) Search(g string) int {
	return sort.Search(len(s), func(k int) bool { return s[k].s >= g })
}

type prob struct {
	s string
	p float64
}

type probs []prob

func (s probs) Len() int           { return len(s) }
func (s probs) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s probs) Less(i, j int) bool { return s[i].s < s[j].s }

func (s probs) SortByNgram() { sort.Sort(s) }

func (s probs) SortByProb() { sort.Sort(byProb{s}) }

func (s probs) SearchNgram(g string) int {
	return sort.Search(len(s), func(k int) bool { return s[k].s >= g })
}

func (s probs) SearchProb(p float64) int {
	return sort.Search(len(s), func(k int) bool { return s[k].p >= p })
}

type byProb struct {
	probs
}

func (s byProb) Less(i, j int) bool {
	return s.probs[i].p < s.probs[j].p
}

func cdf(n int, p func(i int) prob) probs {
	prs := make(probs, n)
	sum := 0.0
	for i := range prs {
		pr := p(i)
		sum += pr.p
		prs[i] = pr
	}
	q := 1.0 / sum
	x := 0.0
	for i, pr := range prs {
		x += pr.p * q
		if x > 1.0 {
			x = 1.0
		}
		prs[i].p = x
	}
	if !sort.IsSorted(byProb{prs}) {
		panic("cdf not sorted")
	}
	return prs
}

func pCDFOfLM(lm ngrams) probs {
	return cdf(len(lm), func(i int) prob {
		return prob{lm[i].s, math.Exp2(lm[i].lgP)}
	})
}

func cCDF(s ngrams) probs {
	return cdf(len(s), func(i int) prob {
		return prob{s[i].s, math.Exp2(s[i].lgQ)}
	})
}

type comap map[string]probs

func comapOfLM(lm ngrams) comap {
	if !sort.IsSorted(lm) {
		panic("lm is not sorted")
	}
	m := make(comap, 26*26)
	for i := 0; i < len(lm); {
		j := i
		g := lm[i].s
		g2 := g[:2]
		z := g2 + "Z"
		i = lm.Search(z)
		if i >= len(lm) || lm[i].s != z {
			panic("unexpected search result")
		}
		i++
		m[g2] = cCDF(lm[j:i])
	}
	return m
}

func (c comap) trigram(g2 string, p float64) string {
	prs := c[g2]
	i := prs.SearchProb(p)
	return prs[i].s
}

var (
	// CDF for normal probabilities
	pcdf probs = pCDFOfLM(englm3)
	// map of two letter conditionals
	cmap comap = comapOfLM(englm3)
)

type Reader struct {
	rnd *rand.Rand
	g3  string
}

func NewReader(src rand.Source) *Reader {
	rnd := rand.New(src)
	i := pcdf.SearchProb(rnd.Float64())
	return &Reader{rnd, pcdf[i].s}
}

func (r *Reader) Read(p []byte) (n int, err error) {
	for i := range p {
		r.g3 = cmap.trigram(r.g3[1:], r.rnd.Float64())
		p[i] = r.g3[2]
	}
	return len(p), nil
}
