package lzbase

// Maximum and minimum values for individual parameters.
const (
	MinLC       = 0
	MaxLC       = 8
	MinLP       = 0
	MaxLP       = 4
	MinPB       = 0
	MaxPB       = 4
	MinDictSize = 1 << 12
	MaxDictSize = 1<<32 - 1
)

// Properties contains the parametes lc, lp and pb.
type Properties byte

// NewProperties returns a new properties value. It verifies the validity of
// the arguments.
func NewProperties(lc, lp, pb int) (p Properties, err error) {
	if err = verifyProperties(lc, lp, pb); err != nil {
		return
	}
	return Properties((pb*5+lp)*9 + lc), nil
}

// LC returns the number of literal context bits.
func (p Properties) LC() int {
	return int(p) % 9
}

// LP returns the number of literal position bits.
func (p Properties) LP() int {
	return (int(p) / 9) % 5
}

// PB returns the number of position bits.
func (p Properties) PB() int {
	return int(p) / 45
}

// verifyProperties checks the argument for any errors.
func verifyProperties(lc, lp, pb int) error {
	if !(MinLC <= lc && lc <= MaxLC) {
		return newError("lc out of range")
	}
	if !(MinLP <= lp && lp <= MaxLC) {
		return newError("lp out of range")
	}
	if !(MinPB <= pb && pb <= MaxPB) {
		return newError("pb out of range")
	}
	return nil
}
