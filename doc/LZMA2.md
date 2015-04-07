% LZMA2 format

The LZMA2 format suppports flushing, parallel encoding or decoding.
Chunks of data that cannot be compressed are copied as such.

## Dictionary Size

LZMA2 requires information about the size of the dictionary. This is
provided by a single byte. 

Bits | Mask | Description
----:|-----:|:------------------------------------------------
 0-5 | 0x3F | Dictionary Size
 6-7 | 0xC0 | Reserved for future use; Must be zero

The dictionary size is encoded with a one-bit mantissa and five-bit
exponent. The smalles dictionary size is 4 KiB and the bigges is 4 GiB
- 1 B.

|Raw Value | Mantissa | Exponent | Dictionary size|
|---------:|---------:|---------:|---------------:|
|        0 |        2 |       11 |          4 KiB |
|        1 |        3 |       11 |          6 KiB |
|        2 |        2 |       12 |          8 KiB |
|        3 |        3 |       12 |         12 KiB |
|      ... |      ... |      ... |            ... |
|       36 |        2 |       29 |       1024 MiB |
|       37 |        3 |       29 |       1536 MiB |
|       38 |        2 |       30 |       2048 MiB |
|       39 |        3 |       30 |       3072 MiB |
|       40 |        2 |       31 |  4096 MiB - 1B |

For test purposes we add the dictionary size byte as first byte of a
LZMA2 stream.

## Chunks

An LZMA2 stream is a sequence of chunks. Each chunk is preceded by a
control byte and other information.

Following the C implementation in the LZMA SDK the control byte can be
described as such:

Chunk header         | Description
:------------------- | :--------------------------------------------------
`00000000`           | End of LZMA2 stream
`00000001 U U`       | Uncompressed chunk, reset dictionary
`00000010 U U`       | Uncompressed chunk, no reset of dictionary
`100uuuuu U U P P`   | LZMA, no reset
`101uuuuu U U P P`   | LZMA, reset state
`110uuuuu U U P P S` | LZMA, reset state, new properties
`111uuuuu U U P P S` | LZMA, reset state, new properties, reset dictionary

The symbols used are described by following table.

A dictionary reset requires always new properties. If this is an
uncompressed chunk the properties need to be provided in the next
compressed chunk. New properties require a reset of the state.

Uncompressed data is written into the dictionary.

Symbol | Description
:----- | :-----------------
u      | unpacked size bit
U      | unpacked size byte
P      | packed size byte
S      | properties byte

The unpacked size and packed size are written in big-endian byte order.
The actual size is one more. So a chunk with 1 byte unpacked data will
store size 0 in the unpacked byte.

The properties byte provides the parameters pb, lc, lp using following
formula:

    S = (pb * 5 + lp) * 9 + lc

For LZMA2 following limitation has been introduced:

    lc + lp <= 4.

The parameters are defined as follows:

Name  | Range  | Description
:---- | :----- | :------------------------------
lc    | [0,8]  | number of literal context bits
lp    | [0,4]  | number of literal pos bits
pb    | [0,4]  | the number of pos bits

