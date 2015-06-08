package main

// translating preset config into lzma params
// default preset 6
// dict_pow2 18, 20, 21, 22, 22, 23, 23, 24, 25, 26
// LC-LP-PB: 3,0,2
// see tukaani xz implementation lzma_encoder_presets
// preset <= 3 0: HC3 1: HC4
// nice_len: 128 : 273
// depths: 4, 8, 24, 48 
// MODE_FAST
// preset > 3: BT4
// nice_len: 4: 16 5: 32 sonst 64
// depth 0
// MODE_NORMAL
