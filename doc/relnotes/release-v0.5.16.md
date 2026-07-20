# Release Notes v0.5.16

This release adds a fallback implementation of term.IsTerminal for unsupported
operating systems including Illumos.

The fallback implemenation always returns false and allows the compilation of
the library. 

It is possible to support Illumos but that would require the import of the x/sys
module, which has security issues. The fixed versions don't support go1.20.

To avoid the complexity I implemented the fallback option for IsTerminal.
