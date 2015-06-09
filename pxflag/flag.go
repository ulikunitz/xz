package pxflag

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

var CommandLine = NewFlagSet(os.Args[0], ExitOnError)

type ErrorHandling int

const (
	ContinueOnError ErrorHandling = iota
	ExitOnError
	PanicOnError
)

type HasArg int

const (
	RequiredArg HasArg = iota
	NoArg
	OptionalArg
)

type Value interface {
	Set(string) error
	Get() interface{}
	String() string
}

type Flag struct {
	Name      string
	Shorthand string
	HasArg    HasArg
	Usage     string
	Value     Value
	DefValue  string
}

type FlagSet struct {
	Usage func()

	name          string
	parsed        bool
	actual        map[string]*Flag
	formal        map[string]*Flag
	shorthand     map[string]*Flag
	args          []string
	output        io.Writer
	errorHandling ErrorHandling
}

func (f *FlagSet) Init(name string, errorHandling ErrorHandling) {
	f.name = name
	f.errorHandling = errorHandling
}

func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	f := new(FlagSet)
	f.Init(name, errorHandling)
	return f
}

func (f *FlagSet) out() io.Writer {
	if f.output == nil {
		return os.Stderr
	}
	return f.output
}

func (f *FlagSet) CounterP(name, shorthand string, value int,
	usage string) *int {
	panic("TODO")
}
func (f *FlagSet) CounterVarP(p *int, name, shorthand string, value int,
	usage string) {
	panic("TODO")
}

type boolValue bool

func newBoolValue(val bool, p *bool) *boolValue {
	*p = val
	return (*boolValue)(p)
}

func (b *boolValue) Get() interface{} {
	return bool(*b)
}

func (b *boolValue) Set(s string) error {
	if s == "" {
		*b = boolValue(true)
	}
	v, err := strconv.ParseBool(s)
	*b = boolValue(v)
	return err
}

func (b *boolValue) String() string {
	return fmt.Sprintf("%t", *b)
}

func (f *FlagSet) BoolP(name, shorthand string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVarP(p, name, shorthand, value, usage)
	return p
}

func BoolP(name, shorthand string, value bool, usage string) *bool {
	return CommandLine.BoolP(name, shorthand, value, usage)
}

func (f *FlagSet) BoolVarP(p *bool, name, shorthand string, value bool,
	usage string) {
	f.VarP(newBoolValue(value, p), name, shorthand, usage, OptionalArg)
}

func BoolVarP(p *bool, name, shorthand string, value bool, usage string) {
	CommandLine.VarP(newBoolValue(value, p), name, shorthand, usage,
		OptionalArg)
}

func (f *FlagSet) VarP(value Value, name, shorthand, usage string, hasArg HasArg) {
	flag := &Flag{Name: name, Usage: usage, Value: value, DefValue: value.String()}
	_, alreadythere := f.formal[name]
	if alreadythere {
		var msg string
		if f.name == "" {
			msg = fmt.Sprintf("flag redefined: %s", name)
		} else {
			msg = fmt.Sprintf("%s flag redefined: %s", f.name, name)
		}
		fmt.Fprintln(f.out(), msg)
		panic(msg)
	}
	if f.formal == nil {
		f.formal = make(map[string]*Flag)
	}
	f.formal[name] = flag
}
