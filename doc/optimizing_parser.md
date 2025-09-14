# Optimizing parser

We are implementing a brute force algorithm.

We search through all matches, determine the longest match and then
calculate the bit size for the matches with decreasing length and check
whether we have a new optimum for a specific position.

We keep a table of blocksize of folloing structs:

* cost as uint32, total bits required for encoding to this position
* len as uint32, if len 1 it's a literal encoded in offset
* offset as uint32

We could encode directly of the table but for now return a block.

How to structure the encoder.
