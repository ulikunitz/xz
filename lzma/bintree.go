package lzma

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"unicode"
)

type node struct {
	// x is the search value
	x uint32
	// p parent node
	p uint32
	// l left child
	l uint32
	// r right child
	r uint32
}

// wordLen is the number of bytes represented by the v field of a node.
const wordLen = 4

type binTree struct {
	node  []node
	hoff  int64
	front uint32
	root  uint32
	x     uint32
}

const null uint32 = 1<<32 - 1

func newBinTree(capacity int) (t *binTree, err error) {
	if capacity < 1 {
		return nil, errors.New(
			"newBinTree: capacity must be larger than zero")
	}
	if int64(capacity) >= int64(null) {
		return nil, errors.New(
			"newBinTree: capacity must less 2^{32}-1")
	}
	t = &binTree{
		node: make([]node, capacity),
		hoff: -int64(wordLen),
		root: null,
	}
	return t, nil
}

func (t *binTree) WriteByte(c byte) error {
	t.x = (t.x << 8) | uint32(c)
	t.hoff++
	if t.hoff < 0 {
		return nil
	}
	v := t.front
	if int64(v) < t.hoff {
		t.remove(v)
	}
	t.node[v].x = t.x
	t.add(v)
	t.front++
	if int64(t.front) >= int64(len(t.node)) {
		t.front = 0
	}
	return nil
}

func (t *binTree) Write(p []byte) (n int, err error) {
	for _, c := range p {
		t.WriteByte(c)
	}
	return len(p), nil
}

func (t *binTree) add(v uint32) {
	vn := &t.node[v]
	vn.l, vn.r = null, null
	if t.root == null {
		t.root = v
		vn.p = null
		return
	}
	x := vn.x
	p := t.root
	for {
		pn := &t.node[p]
		if x <= pn.x {
			if pn.l == null {
				pn.l = v
				vn.p = p
				return
			}
			p = pn.l
		} else {
			if pn.r == null {
				pn.r = v
				vn.p = p
				return
			}
			p = pn.r
		}
	}
}

func (t *binTree) parent(v uint32) (p uint32, ptr *uint32) {
	if t.root == v {
		return null, &t.root
	}
	p = t.node[v].p
	if t.node[p].l == v {
		ptr = &t.node[p].l
	} else {
		ptr = &t.node[p].r
	}
	return
}

func (t *binTree) remove(v uint32) {
	vn := &t.node[v]
	p, ptr := t.parent(v)
	l, r := vn.l, vn.r
	if l == null {
		*ptr = r
		if r != null {
			t.node[r].p = p
		}
		return
	}
	if r == null {
		*ptr = l
		t.node[l].p = p
		return
	}

	// search the in-order predecessor u
	un := &t.node[l]
	ur := un.r
	if ur == null {
		// we move the left child of v, u, up
		un.r = r
		t.node[r].p = l
		un.p = p
		*ptr = l
		return
	}
	var u uint32
	for {
		u = ur
		ur = t.node[u].r
		if ur == null {
			break
		}
	}
	// replace u with ul
	un = &t.node[u]
	ul := un.l
	up := un.p
	t.node[up].r = ul
	if ul != null {
		t.node[ul].p = up
	}

	// replace v by u
	un.l, un.r = l, r
	t.node[l].p = u
	t.node[r].p = u
	*ptr = u
	un.p = p
}

func (t *binTree) find(x uint32) (v uint32) {
	if t.root == null {
		return null
	}
	p := t.root
	for {
		pn := &t.node[p]
		if x <= pn.x {
			if x == pn.x || pn.l == null {
				return p
			}
			p = pn.l
		} else {
			if pn.r == null {
				return p
			}
			p = pn.r
		}
	}
}

func xval(a []byte) uint32 {
	x := uint32(a[0]) << 24
	x |= uint32(a[1]) << 16
	x |= uint32(a[2]) << 8
	x |= uint32(a[3])
	return x
}

func dumpX(x uint32) string {
	a := make([]byte, 4)
	for i := 0; i < 4; i++ {
		c := byte(x >> uint((3-i)*8))
		if unicode.IsGraphic(rune(c)) {
			a[i] = c
		} else {
			a[i] = '.'
		}
	}
	return string(a)
}

func writeIndent(w io.Writer, indent int) {
	for i := 0; i < indent; i++ {
		fmt.Fprint(w, " ")
	}
}

func (t *binTree) dumpNode(w io.Writer, v uint32, indent int) {
	writeIndent(w, indent)
	if v == null {
		fmt.Fprintln(w, "null")
		return
	}
	vn := &t.node[v]
	if vn.p == null {
		fmt.Fprintf(w, "node %d %q parent null\n", v, dumpX(vn.x))
	} else {
		fmt.Fprintf(w, "node %d %q parent %d\n", v, dumpX(vn.x), vn.p)
	}
	t.dumpNode(w, vn.l, indent+2)
	t.dumpNode(w, vn.r, indent+2)
}

func (t *binTree) dump(w io.Writer) error {
	bw := bufio.NewWriter(w)
	t.dumpNode(bw, t.root, 0)
	return bw.Flush()
}
