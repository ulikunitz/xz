package gflag

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
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
	Value      Value
}

type line struct {
	flags string
	usage string
}

func lineFlags(name, shorthands, defaultValue string) string {
	buf := new(bytes.Buffer)
	if shorthands != "" {
		for i, r := range shorthands {
			if i > 0 {
				fmt.Fprint(buf, ", ")
			}
			fmt.Fprintf(buf, "-%c", r)
		}
	}
	if name != "" {
		if buf.Len() > 0 {
			fmt.Fprintf(buf, ", ")
		}
		fmt.Fprint(buf, "--", name)
		if defaultValue != "" {
			fmt.Fprintf(buf, "=%s", defaultValue)
		}
	}
	return buf.String()
}

type lines []line

func writeLines(w io.Writer, ls lines) (n int, err error) {
	l := make(lines, len(ls))
	copy(l, ls)
	sort.Sort(l)
	maxLenFlags := 0
	for _, line := range l {
		k := len(line.flags)
		if k > maxLenFlags {
			maxLenFlags = k
		}
	}
	for _, line := range l {
		format := fmt.Sprintf("  %%-%ds  %%s\n", maxLenFlags)
		var k int
		k, err = fmt.Fprintf(w, format, line.flags, line.usage)
		n += k
		if err != nil {
			return
		}
	}
	return
}

func (l lines) Len() int           { return len(l) }
func (l lines) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l lines) Less(i, j int) bool { return l[i].flags < l[j].flags }

type FlagSet struct {
	Usage func()

	name          string
	parsed        bool
	actual        map[string]*Flag
	formal        map[string]*Flag
	lines         lines
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

func defaultUsage(f *FlagSet) {
	if f.name == "" {
		fmt.Fprintf(f.out(), "Usage:\n")
	} else {
		fmt.Fprintf(f.out(), "Usage of %s:\n", f.name)
	}
	f.PrintDefaults()
}

var Usage = func() {
	fmt.Fprintf(CommandLine.out(), "Usage of %s:\n", os.Args[0])
	PrintDefaults()
}

func (f *FlagSet) usage() {
	if f.Usage == nil {
		if f == CommandLine {
			Usage()
		} else {
			defaultUsage(f)
		}
	} else {
		f.Usage()
	}
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
		fmt.Fprintf(f.out(), "%s: %s\n", f.name, err)
		f.usage()
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

func (f *FlagSet) PrintDefaults() {
	_, err := writeLines(f.out(), f.lines)
	if err != nil {
		f.panicf("writeLines error %s", err)
	}
}

func PrintDefaults() {
	CommandLine.PrintDefaults()
}

func (f *FlagSet) out() io.Writer {
	if f.output == nil {
		return os.Stderr
	}
	return f.output
}

func (f *FlagSet) SetOutput(w io.Writer) {
	f.output = w
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

func (f *FlagSet) VarP(value Value, name, shorthands string, hasArg HasArg) {
	flag := &Flag{
		Name:       name,
		Shorthands: shorthands,
		Value:      value,
		HasArg:     hasArg,
	}

	if flag.Name == "" && flag.Shorthands == "" {
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

func VarP(value Value, name, shorthands string, hasArg HasArg) {
	CommandLine.VarP(value, name, shorthands, hasArg)
}

func (f *FlagSet) Var(value Value, name string, hasArg HasArg) {
	shorthands := ""
	if len(name) == 1 {
		shorthands = name
		name = ""
	}
	f.VarP(value, name, shorthands, hasArg)
}

func Var(value Value, name string, hasArg HasArg) {
	CommandLine.Var(value, name, hasArg)
}

func (f *FlagSet) addLine(l line) {
	if l.flags == "" {
		f.panicf("no flags for %q", l.usage)
	}
	f.lines = append(f.lines, l)
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

func boolLine(name, shorthands string, value bool, usage string) line {
	defaultValue := ""
	if value {
		defaultValue = "true"
	}
	return line{lineFlags(name, shorthands, defaultValue), usage}
}

func (f *FlagSet) BoolVarP(p *bool, name, shorthands string, value bool, usage string) {
	f.addLine(boolLine(name, shorthands, value, usage))
	f.VarP(newBoolValue(value, p), name, shorthands, OptionalArg)
}

func (f *FlagSet) BoolP(name, shorthands string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVarP(p, name, shorthands, value, usage)
	return p
}

func BoolP(name, shorthands string, value bool, usage string) *bool {
	return CommandLine.BoolP(name, shorthands, value, usage)
}

func BoolVarP(p *bool, name, shorthands string, value bool, usage string) {
	CommandLine.BoolVarP(p, name, shorthands, value, usage)
}

func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	f.addLine(boolLine(name, "", value, usage))
	f.Var(newBoolValue(value, p), name, OptionalArg)
}

func BoolVar(p *bool, name string, value bool, usage string) {
	CommandLine.BoolVar(p, name, value, usage)
}

func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVar(p, name, value, usage)
	return p
}

func Bool(name string, value bool, usage string) *bool {
	return CommandLine.Bool(name, value, usage)
}

type intValue int

func newIntValue(val int, p *int) *intValue {
	*p = val
	return (*intValue)(p)
}

func (n *intValue) Get() interface{} {
	return int(*n)
}

func (n *intValue) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 0)
	*n = intValue(v)
	return err
}

func (n *intValue) Update() {
	(*n)++
}

func (n *intValue) String() string {
	return fmt.Sprintf("%d", *n)
}

func counterLine(name, shorthands, usage string) line {
	return line{lineFlags(name, shorthands, ""), usage}
}

func (f *FlagSet) CounterVarP(p *int, name, shorthands string, value int, usage string) {
	f.addLine(counterLine(name, shorthands, usage))
	f.VarP(newIntValue(value, p), name, shorthands, OptionalArg)
}

func CounterVarP(p *int, name, shorthands string, value int, usage string) {
	CommandLine.CounterVarP(p, name, shorthands, value, usage)
}

func (f *FlagSet) CounterP(name, shorthands string, value int, usage string) *int {
	p := new(int)
	f.CounterVarP(p, name, shorthands, value, usage)
	return p
}

func CounterP(name, shorthands string, value int, usage string) *int {
	return CommandLine.CounterP(name, shorthands, value, usage)
}

func (f *FlagSet) CounterVar(p *int, name string, value int, usage string) {
	f.addLine(counterLine(name, "", usage))
	f.Var(newIntValue(value, p), name, OptionalArg)
}

func CounterVar(p *int, name string, value int, usage string) {
	CommandLine.CounterVar(p, name, value, usage)
}

func (f *FlagSet) Counter(name string, value int, usage string) *int {
	p := new(int)
	f.CounterVar(p, name, value, usage)
	return p
}

func Counter(name string, value int, usage string) *int {
	return CommandLine.Counter(name, value, usage)
}

func intLine(name, shorthands string, value int, usage string) line {
	defaultValue := ""
	if value != 0 {
		defaultValue = fmt.Sprintf("%d", value)
	}
	return line{lineFlags(name, shorthands, defaultValue), usage}
}

func (f *FlagSet) IntVarP(p *int, name, shorthands string, value int, usage string) {
	f.addLine(intLine(name, shorthands, value, usage))
	f.VarP(newIntValue(value, p), name, shorthands, RequiredArg)
}

func IntVarP(p *int, name, shorthands string, value int, usage string) {
	CommandLine.IntVarP(p, name, shorthands, value, usage)
}

func (f *FlagSet) IntP(name, shorthands string, value int, usage string) *int {
	p := new(int)
	f.IntVarP(p, name, shorthands, value, usage)
	return p
}

func IntP(name, shorthands string, value int, usage string) *int {
	return CommandLine.IntP(name, shorthands, value, usage)
}

func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {
	f.addLine(intLine(name, "", value, usage))
	f.Var(newIntValue(value, p), name, RequiredArg)
}

func IntVar(p *int, name string, value int, usage string) {
	CommandLine.IntVar(p, name, value, usage)
}

func (f *FlagSet) Int(name string, value int, usage string) *int {
	p := new(int)
	f.IntVar(p, name, value, usage)
	return p
}

func Int(name string, value int, usage string) *int {
	return CommandLine.Int(name, value, usage)
}
