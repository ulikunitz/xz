package gflag

import "testing"

func TestFlagSet_Bool(t *testing.T) {
	f := NewFlagSet("Bool", ContinueOnError)
	a := f.Bool("test-a", false, "")
	b := f.BoolP("test-b", "b", true, "")

	err := f.Parse([]string{"--test-a", "-b", "false"})
	if err != nil {
		t.Fatalf("f.Parse error %s", err)
	}

	if *a != true {
		t.Errorf("*a is %t; want %t", *a, true)
	}
	if *b != false {
		t.Errorf("*b is %t; want %t", *b, false)
	}
}
