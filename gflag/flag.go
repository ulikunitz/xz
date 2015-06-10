package gflag

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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
	Update()
	Get() interface{}
	String() string
}

type Flag struct {
	Name       string
	Shorthands string
	HasArg     HasArg
	Usage      string
	Value      Value
	DefValue   string
}

type FlagSet struct {
	Usage func()

	name          string
	parsed        bool
	actual        map[string]*Flag
	formal        map[string]*Flag
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

func (f *FlagSet) Arg(i int) string {
	if !(0 <= i && i < len(f.args)) {
		return ""
	}
	return f.args[i]
}

func Arg(i int) string {
	return CommandLine.Arg(i)
}

func (f *FlagSet) Args() []string { return f.args }

func Args() []string { return CommandLine.args }

func (f *FlagSet) NArg() int { return len(f.args) }

func NArg() int { return len(CommandLine.args) }

func Parsed() bool {
	return CommandLine.parsed
}

func (f *FlagSet) Parsed() bool {
	return f.parsed
}

func Parse() {
	// errors are ignored because CommandLine is set on ExitOnError
	CommandLine.Parse(os.Args[1:])
}

func (f *FlagSet) lookupLongOption(name string) (flag *Flag, err error) {
	if len(name) < 2 {
		f.panicf("%s is not a long option", name)
	}
	var ok bool
	if flag, ok = f.formal[name]; !ok {
		return nil, fmt.Errorf("long option %s is unsupported", name)
	}
	if flag.Name != name {
		f.panicf("got %s flag; want %s flag", flag.Name, name)
	}
	return flag, nil
}

func (f *FlagSet) lookupShortOption(r rune) (flag *Flag, err error) {
	var ok bool
	name := string([]rune{r})
	if flag, ok = f.formal[name]; !ok {
		return nil, fmt.Errorf("short option %s is unsupported", name)
	}
	if !strings.ContainsRune(flag.Shorthands, r) {
		f.panicf("flag supports shorthands %q; but doesn't contain %s",
			flag.Shorthands, name)
	}
	return flag, nil
}

func (f *FlagSet) processExtraFlagArg(flag *Flag, i int) error {
	if flag.HasArg == NoArg {
		// no argument required
		flag.Value.Update()
		return nil
	}
	if i < len(f.args) {
		arg := f.args[i]
		if len(arg) == 0 || arg[0] != '-' {
			f.removeArg(i)
			return flag.Value.Set(arg)
		}
	}
	// no argument
	if flag.HasArg == RequiredArg {
		return fmt.Errorf("no argument present")
	}
	// flag.HasArg == OptionalArg
	flag.Value.Update()
	return nil
}

func (f *FlagSet) removeArg(i int) {
	copy(f.args[i:], f.args[i+1:])
	f.args = f.args[:len(f.args)-1]
}

func (f *FlagSet) parseArg(i int) (next int, err error) {
	arg := f.args[i]
	if len(arg) < 2 || arg[0] != '-' {
		return i + 1, nil
	}
	if arg[1] == '-' {
		// argument starts with --
		f.removeArg(i)
		if len(arg) == 2 {
			// argument is --; remove it and ignore all
			// following arguments
			return len(f.args), nil
		}
		arg = arg[2:]
		flagArg := strings.SplitN(arg, "=", 2)
		flag, err := f.lookupLongOption(flagArg[0])
		if err != nil {
			return i, err
		}
		// case 1: no equal sign
		if len(flagArg) == 1 {
			err = f.processExtraFlagArg(flag, i)
			return i, err
		}
		// case 2: equal sign
		if flag.HasArg == NoArg {
			err = fmt.Errorf("option %s doesn't support argument",
				arg)
		} else {
			err = flag.Value.Set(flagArg[1])
		}
		return i, err
	}
	// short options
	f.removeArg(i)
	arg = arg[1:]
	for _, r := range arg {
		flag, err := f.lookupShortOption(r)
		if err != nil {
			return i, err
		}
		if err = f.processExtraFlagArg(flag, i); err != nil {
			return i, err
		}
	}
	return i, nil
}

func (f *FlagSet) Parse(arguments []string) error {
	f.parsed = true
	f.args = arguments
	for i := 0; i < len(f.args); {
		var err error
		i, err = f.parseArg(i)
		if err == nil {
			continue
		}
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			os.Exit(2)
		case PanicOnError:
			panic(err)
		}
	}
	return nil
}

func (f *FlagSet) out() io.Writer {
	if f.output == nil {
		return os.Stderr
	}
	return f.output
}

func (f *FlagSet) CounterP(name, shorthands string, value int,
	usage string) *int {
	panic("TODO")
}
func (f *FlagSet) CounterVarP(p *int, name, shorthands string, value int,
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
	v, err := strconv.ParseBool(s)
	*b = boolValue(v)
	return err
}

func (b *boolValue) Update() {
	*b = true
}

func (b *boolValue) String() string {
	return fmt.Sprintf("%t", *b)
}

func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	return f.BoolP(name, "", value, usage)
}

func (f *FlagSet) BoolP(name, shorthands string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVarP(p, name, shorthands, value, usage)
	return p
}

func Bool(name string, value bool, usage string) *bool {
	return CommandLine.BoolP(name, "", value, usage)
}

func BoolP(name, shorthands string, value bool, usage string) *bool {
	return CommandLine.BoolP(name, shorthands, value, usage)
}

func (f *FlagSet) BoolVarP(p *bool, name, shorthands string, value bool,
	usage string) {
	f.VarP(newBoolValue(value, p), name, shorthands, usage, OptionalArg)
}

func BoolVarP(p *bool, name, shorthands string, value bool, usage string) {
	CommandLine.VarP(newBoolValue(value, p), name, shorthands, usage,
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

func VarP(value Value, name, shorthands, usage string, hasArg HasArg) {
	CommandLine.VarP(value, name, shorthands, usage, hasArg)
}

func (f *FlagSet) setFormal(name string, flag *Flag) {
	if name == "" {
		f.panicf("no support for empty name strings")
	}
	if _, alreadythere := f.formal[name]; alreadythere {
		f.panicf("flag redefined: %s", flag.Name)
	}
	if f.formal == nil {
		f.formal = make(map[string]*Flag)
	}
	f.formal[name] = flag
}

func (f *FlagSet) VarP(value Value, name, shorthands, usage string, hasArg HasArg) {
	flag := &Flag{
		Name:       name,
		Shorthands: shorthands,
		Usage:      usage,
		Value:      value,
		DefValue:   value.String(),
		HasArg:     hasArg,
	}

	if flag.Name == "" && flag.Shorthands != "" {
		f.panicf("flag with no name or shorthands")
	}
	if len(flag.Name) == 1 {
		f.panicf("flag has single character name %q; use shorthands",
			flag.Name)
	}
	if flag.Name != "" {
		f.setFormal(flag.Name, flag)
	}
	if flag.Shorthands != "" {
		for _, r := range flag.Shorthands {
			name := string([]rune{r})
			f.setFormal(name, flag)
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
	shorthands := ""
	if len(name) == 1 {
		shorthands = name
		name = ""
	}
	f.VarP(value, name, shorthands, usage, hasArg)
}
