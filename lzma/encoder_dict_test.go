package lzma

import (
	"testing"
)

func TestNewEncoderDict(t *testing.T) {
	_, err := newEncoderDict(20, 10)
	if err != nil {
		t.Fatalf("newEncoderDict(): error %s", err)
	}
}
