package lzb

// TODO 
// 
// - write hashDict
// - write syncHashDict (sets WriterLimit)
// - write seperate OpGenerator type that contains a hashDict 
//     + provide an interface for the OpGenerator (we will have multiple
//       implementations)
//     + support transfer of state from hashDict to syncHashDict
//     + do only simple greedy at first
// - write fills buffer until full + compression is started at the very
//   end
// - ops that hit buf.top will not be used all others will

type Writer struct {
	State *State
	re *rangeEncoder
	closed bool
}
