package lzma

var ntz32Table = [32]int8{
	0, 1, 2, 24, 3, 19, 6, 25,
	22, 4, 20, 10, 16, 7, 12, 26,
	31, 23, 18, 5, 21, 9, 15, 11,
	30, 17, 8, 14, 29, 13, 28, 27}

func ntz32(x uint32) int {
	if x == 0 {
		return 32
	}
	x = (x & -x) * 0x04d7651f
	return int(ntz32Table[x>>27])
}
