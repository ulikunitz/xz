package main

// I cannot use the preset config from the Tukaani project directly,
// because I don't have two algorithm modes and can't support parameters
// like nice_len or depth. So at this point in time I stay with the
// dictionary sizes the default combination of (LC,LP,LB) = (3,0,2).
// The default preset is 6.
// Following list provides exponents of two for the dictionary sizes:
// 18, 20, 21, 22, 22, 23, 23, 24, 25, 26.

func processLZMA(opts options, arg string) error {
	// TODO: signal handling?
	// use deferred function for closure of output depending
	// on the state of the error variable

	// create buffered input reader
	// create buffered output writer
	// create lzma filter
	// copy data
	// assuming no error
	// close output
	// close input
	// rename output to correct file
	// remove input file if not kept and not stdin
	panic("TODO")
}
