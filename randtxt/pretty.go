package randtxt

import (
	"bufio"
	"io"
	"unicode"
)

type GroupReader struct {
	R             io.ByteReader
	GroupsPerLine int
	off           int64
	eof           bool
}

func NewGroupReader(r io.Reader) *GroupReader {
	return &GroupReader{R: bufio.NewReader(r)}
}

func (r *GroupReader) Read(p []byte) (n int, err error) {
	if r.eof {
		return 0, io.EOF
	}
	if r.GroupsPerLine < 1 {
		r.GroupsPerLine = 8
	}
	lineLen := int64(r.GroupsPerLine * 6)
	var c byte
	for i := range p {
		switch {
		case r.off%lineLen == lineLen-1:
			if i+1 == len(p) && len(p) > 1 {
				return i, nil
			}
			c = '\n'
		case r.off%6 == 5:
			if i+1 == len(p) && len(p) > 1 {
				return i, nil
			}
			c = ' '
		default:
			c, err = r.R.ReadByte()
			if err == io.EOF {
				r.eof = true
				if i > 0 {
					switch p[i-1] {
					case ' ':
						p[i-1] = '\n'
						fallthrough
					case '\n':
						return i, io.EOF
					}
				}
				p[i] = '\n'
				return i + 1, io.EOF
			}
			if err != nil {
				return i, err
			}
			switch {
			case c == ' ':
				c = '_'
			case !unicode.IsPrint(rune(c)):
				c = '-'
			}
		}
		p[i] = c
		r.off++
	}
	return len(p), nil
}
