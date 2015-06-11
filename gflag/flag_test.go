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

	if f.NArg() != 0 {
		t.Errorf("f.NArg() is %d; want %d", f.NArg(), 0)
	}
}

func TestFlagSet_Counter(t *testing.T) {
	f := NewFlagSet("Bool", ContinueOnError)
	a := f.Counter("test-a", 0, "")
	b := f.CounterP("test-b", "b", 0, "")
	err := f.Parse([]string{"--test-a=3", "-b", "5", "--test-a", "-b"})
	if err != nil {
		t.Fatalf("f.Parse error %s", err)
	}

	if *a != 4 {
		t.Errorf("*a is %d; want %d", *a, 4)
	}
	if *b != 6 {
		t.Errorf("*b is %d; want %d", *b, 6)
	}

	if f.NArg() != 0 {
		t.Errorf("f.NArg() is %d; want %d", f.NArg(), 0)
	}
}
