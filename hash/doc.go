/*
Package hash provides rolling hashes.

Rolling hashes have to be used for maintaining the positions of n-byte
sequences in the dictionary buffer.

The package provides currently the Rabin-Karp rolling hash and a Cyclic
Polynomial hash. Both support the Hashes method to be used with an interface.
*/
package hash
