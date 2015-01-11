package lzma

import (
	"testing"
)

func TestVerify(t *testing.T) {
	err := verifyProperties(&Default)
	if err != nil {
		t.Errorf("verifyProperties(&defaultProperties) error %s", err)
	}
	err = verifyProperties(&Properties{})
	if err == nil {
		t.Fatal("verifyProperties(&Properties{}) no error")
	}
	t.Logf("verifyProperties(&Properties{}) error %s", err)
}
