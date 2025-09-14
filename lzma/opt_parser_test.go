package lzma

import "testing"

func TestBucketHash(t *testing.T) {
	var h bucketHash
	if err := h.init(3, 16); err != nil {
		t.Fatal(err)
	}
}
