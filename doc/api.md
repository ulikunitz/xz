# Thoughs about the API and Module structure

During the development of the xz library I faced multiple questions
regarding the design of the interface of the library and I struggled
with benchmarking.

Here is what I found even though my thinking is still developing.

## Configuration

It is obvious that methods SetDefaults and Verify on configuration
structures, don't belong into the hand of the users. These are helper
methods that have local meaning.

Configuration knobs must help user's targets, but I have here two
problems there are sometimes multiple objectives. One objective is to
make it easy for a consumer of the API and provide a minimum set of
knobs. On the other hand we may want to reprogram bad interfaces and
need all the knobs.

Therefore I suggest following structure:

```
NewWriter(z io.Writer) (w io.WriteFlushCloser, err error)

NewWriterPreset(z io.Writer, preset int,  workers int) (w io.WriterFlushCloser, err error)

NewWriterConfig(z io.Writer, cfg WriterConfig) (w io.WriterFlushCloser,
err error)

NewReader(z io.Reader) (w io.WriterFlushCloser, err error) (r io.ReaderCloser, err error)

NewReaderConfig(z io.Reader, cfg ReaderConfig) (r io.ReaderCloser, err error)
```

The presents provide methods from 0 to 10. With increasing numbers more
cpu time and memory would be used to achieve higher compression rate.

After experimenting I came to the conclusion that the configuration
structures should be flat or substructures have an easy way to be
generated.

### lz package

The lz package provides one DecoderBuffer for decoding and a number of
methods to compute Lempel-Ziv sequences. The methods are dominated by
the data structures for finding matches in the dictionary window. That
leads me to following design. The interface for the SeqBuffer should be
reduced to the staff that is actually used. Each method should have a
short name that provides all parameters for the method. Examples are
hs3-10 or bdhs36-12-14 for the BackDoubleHash.

```
type SeqConfig interface {
    NewSequencer() (s Sequencer, err error)
    SetDefaults()
    Verify() error
    SeqBufConfig() *SeqBufConfig
    Method() string
}

func ParseMethod(name string) (cfg SecConfig, err error)

// AsMethod converts the method name to a Method instance. If the name
// is invalid or unsupported the functions panics with the error from
// ParseMathod.
func SeqConfig(method string) SeqConfig

// further methods: DoubleHash, BackHash, BackDoubleHash, GreedySuffixArray,
//                  BucketHash

func PresetSecConfig(p Params) SecConfig
```

## Benchmarking

I had the idea that I could maintain an independent benchmarking tool
and I had a very comprehensive tool for benchmarking the lz package and
the various sequencers I had implemented, but I found out that I don't
maintain that tool if the interfaces change, since the code is another
module. So the tuning tools that create the presets must be included in
the modules for the compression methods. The concern regarding the large
size of the corporas can be addressed by moving them into its own
corpora module.

## Some basic thoughs about lz

WindowSize W is only a constraint on the sequencer not to generate a
sequence with an offset larger than the window size.

BlockSize B is also a parameter for the Sequence method and has nothing to
do with the buffer.

Optionally we have the requirement that the full window must be
available for the hiher level encoder in the LZMA case. That means
shrink size S must equal the window size W.

The buffer has a shrink size S and a buffer size U.

W >= 0
B > 0 // otherwise we are not making progress in the sequencer.

The buffer tracks w the end of the window and it is advanced by the
sequencer. It also tracks pos for the start position of the buffer.

We are using the sequencer in two modes. Mode 1 we have a byte slice and
the sequencer works on the byte slice. We can call it static mode and
the streaming mode, where data will be added during whole sequencing is
in progress. Shrink may be called if data is added to the buffer.










