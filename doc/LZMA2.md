# LZMA2 Format

LZMA2 is a container of chunks.

Each chunk is lead by a control byte.

If bit 7 is not set, then there are only three cases:

0x00 End of Stream


0x00 - end of LZMA2 stream (no further data)
0x01 - uncompressed chunk



control             byte
uncompressed size   uint16 (real - 1)
compressed size     uint16 (real - 1)

control & 0x1f contains upper bits of uncompressed size


0x00 - end of the LZMA2 stream
0x01 - properties required; reset of LZMA stream
0xe0 - also ok

needDictReset set

> 0x80 - LZMA chunk
  
