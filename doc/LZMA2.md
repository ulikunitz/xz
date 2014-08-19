# LZMA2 Format

LZMA2 is a container of chunks. Each chunk is lead by a control byte.

Following the C implementation in the LZMA SDK the control byte can be
described as such:

-------------------- ---------------------------------------------------
`00000000`           End of LZMA2 stream
`00000001 U U`       Uncompressed chunk; reset dictionary
`00000010 U U`       Uncompressed chunk; no reset of dictionary
`100uuuuu U U P P`   LZMA; no reset
`101uuuuu U U P P`   LZMA; reset state
`110uuuuu U U P P S` LZMA; reset state; new properties
`111uuuuu U U P P S` LZMA; reset state; new properties; reset dictionary
-------------------- ---------------------------------------------------

The symbols used are described by following table.

-- -------------------
 u unpacked size bit
 U unpacked size byte
 P packed size byte
 S properties
-- -------------------
