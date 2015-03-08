# LZMA2 Format

LZMA2 is a container of chunks. Each chunk is lead by a control byte.

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

