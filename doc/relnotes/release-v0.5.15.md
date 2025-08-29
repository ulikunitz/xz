# Release Notes v0.5.15

This release addresses a problem for 32-bit platforms by setting the
DictCap to 1<<31, which doesn't fit an int on such platforms. I set the
limit to 1<<31-1 now, which is MaxInt on those platforms.

All tests with GOARCH=386 are compiling and running successfully.

