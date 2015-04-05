package lzma

import (
	"testing"
)

func TestVerify(t *testing.T) {
	err := verifyParameters(&Default)
	if err != nil {
		t.Errorf("verifyParameters(&defaultParameters) error %s", err)
	}
	err = verifyParameters(&Parameters{})
	if err == nil {
		t.Fatal("verifyParameters(&Parameters{}) no error")
	}
	t.Logf("verifyParameters(&Parameters{}) error %s", err)
}
