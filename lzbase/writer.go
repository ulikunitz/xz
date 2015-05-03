package lzbase

import "io"

// Writer supports the creation of an LZMA stream.
type Writer struct {
	enc OpEncoder
}

// NewWriter creates a writer using the state. The argument eos defines whether
// an explicit end-of-stream marker will be written. The writer will be limited
// by MaxLimit (2^63 - 1), which is practically unlimited.
func NewWriter(w io.Writer, state *WriterState, eos bool) *Writer {
	return &Writer{OpEncoder{
		State: state,
		dict:  state.WriterDict(),
		re:    newRangeEncoder(w),
		eos:   eos}}
}

// Write moves data into the internal buffer and triggers its compression. Note
// that beside the data held back to enable a large match all data will be be
// compressed.
func (bw *Writer) Write(p []byte) (n int, err error) {
	end := bw.enc.dict.end + int64(len(p))
	if end < 0 {
		panic("end counter overflow")
	}
	for n < len(p) {
		k, err := bw.enc.dict.Write(p[n:])
		n += k
		if err != nil && err != errAgain {
			return n, err
		}
		if err = bw.process(0); err != nil {
			return n, err
		}
	}
	return n, nil
}

// Close terminates the LZMA stream. It doesn't close the underlying writer
// though and leaves it alone. In some scenarios explicit closing of the
// underlying writer is required.
func (bw *Writer) Close() error {
	var err error
	if err = bw.process(allData); err != nil {
		return err
	}
	return bw.enc.Close()
}

// The allData flag tells the process method that all data must be processed.
const allData = 1

// process encodes the data written into the dictionary buffer. The allData
// flag requires all data remaining in the buffer to be encoded.
func (bw *Writer) process(flags int) error {
	panic("TODO")
}
