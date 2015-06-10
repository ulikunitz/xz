package gflag

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
	shorthand     map[rune]*Flag
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

func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	return f.BoolP(name, "", value, usage)
}

func (f *FlagSet) BoolP(name, shorthand string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVarP(p, name, shorthand, value, usage)
	return p
}

func Bool(name string, value bool, usage string) *bool {
	return CommandLine.BoolP(name, "", value, usage)
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

func BoolVar(p *bool, name string, value bool, usage string) {
	CommandLine.BoolVarP(p, name, "", value, usage)
}

func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	f.BoolVarP(p, name, "", value, usage)
}

func (f *FlagSet) panicf(format string, values ...interface{}) {
	var msg string
	if f.name == "" {
		msg = fmt.Sprintf(format, values...)
	} else {
		v := make([]interface{}, 1+len(values))
		v[0] = f.name
		copy(v[1:], values)
		msg = fmt.Sprintf("%s "+format, v...)
	}
	fmt.Fprintln(f.out(), msg)
	panic(msg)
}

func VarP(value Value, name, shorthand, usage string, hasArg HasArg) {
	CommandLine.VarP(value, name, shorthand, usage, hasArg)
}

func (f *FlagSet) VarP(value Value, name, shorthand, usage string, hasArg HasArg) {
	flag := &Flag{
		Name:      name,
		Shorthand: shorthand,
		Usage:     usage,
		Value:     value,
		DefValue:  value.String(),
	}

	if flag.Name == "" && flag.Shorthand != "" {
		f.panicf("flag with no name or shorthand")
	}

	if flag.Name != "" {
		_, alreadythere := f.formal[name]
		if alreadythere {
			f.panicf("flag redefined: %s", flag.Name)
		}
		if f.formal == nil {
			f.formal = make(map[string]*Flag)
		}
		f.formal[name] = flag
	}
	if flag.Shorthand != "" {
		if f.shorthand == nil {
			f.shorthand = make(map[rune]*Flag)
		}
		for _, r := range flag.Shorthand {
			f.shorthand[r] = flag
		}
	}
}

func Var(value Value, name, usage string) {
	CommandLine.Var(value, name, usage)
}

func (f *FlagSet) Var(value Value, name, usage string) {
	hasArg := RequiredArg
	switch value.(type) {
	case *boolValue:
		hasArg = OptionalArg
	}
	f.VarP(value, name, "", usage, hasArg)
}
