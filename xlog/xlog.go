package xlog

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	Ldate = 1 << iota
	Ltime
	Lmicroseconds
	Llongfile
	Lshortfile
	Lnopanic
	Lnofatal
	Lnowarn
	Lnoprint
	Lnodebug
	Lstdflags = Ldate | Ltime
)

type Logger struct {
	mu     sync.Mutex
	prefix string
	flag   int
	out    io.Writer
	buf    []byte
}

func New(out io.Writer, prefix string, flag int) *Logger {
	return &Logger{out: out, prefix: prefix, flag: flag}
}

var std = New(os.Stderr, "", Lstdflags)

func itoa(buf *[]byte, i int, wid int) {
	var u uint = uint(i)
	if u == 0 && wid <= 1 {
		*buf = append(*buf, '0')
		return
	}
	var b [32]byte
	bp := len(b)
	for ; u > 0 || wid > 0; u /= 10 {
		bp--
		wid--
		b[bp] = byte(u%10) + '0'
	}
	*buf = append(*buf, b[bp:]...)
}

func (l *Logger) formatHeader(t time.Time, file string, line int) {
	l.buf = append(l.buf, l.prefix...)
	if l.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		if l.flag&Ldate != 0 {
			year, month, day := t.Date()
			itoa(&l.buf, year, 4)
			l.buf = append(l.buf, '-')
			itoa(&l.buf, int(month), 2)
			l.buf = append(l.buf, '-')
			itoa(&l.buf, day, 2)
			l.buf = append(l.buf, ' ')
		}
		if l.flag&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(&l.buf, hour, 2)
			l.buf = append(l.buf, ':')
			itoa(&l.buf, min, 2)
			l.buf = append(l.buf, ':')
			itoa(&l.buf, sec, 2)
			if l.flag&Lmicroseconds != 0 {
				l.buf = append(l.buf, '.')
				itoa(&l.buf, t.Nanosecond()/1e3, 6)
			}
			l.buf = append(l.buf, ' ')
		}
	}
	if l.flag&(Lshortfile|Llongfile) != 0 {
		if l.flag&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}
		l.buf = append(l.buf, file...)
		l.buf = append(l.buf, ':')
		itoa(&l.buf, line, -1)
		l.buf = append(l.buf, ": "...)
	}
}

func (l *Logger) Output(calldepth int, s string) error {
	now := time.Now()
	var file string
	var line int
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.flag&(Lshortfile|Llongfile) != 0 {
		l.mu.Unlock()
		var ok bool
		_, file, line, ok = runtime.Caller(calldepth)
		if !ok {
			file = "???"
			line = 0
		}
		l.mu.Lock()
	}
	l.buf = l.buf[:0]
	l.formatHeader(now, file, line)
	l.buf = append(l.buf, s...)
	if len(s) == 0 || s[len(s)-1] != '\n' {
		l.buf = append(l.buf, '\n')
	}
	_, err := l.out.Write(l.buf)
	return err
}

func (l *Logger) Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	// no need for locking because integer access is atomic
	if l.flag&Lnopanic == 0 {
		l.Output(2, s)
	}
	panic(s)
}

func Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	// no need for locking because integer access is atomic
	if std.flag&Lnopanic == 0 {
		std.Output(2, s)
	}
	panic(s)
}

func (l *Logger) Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	if l.flag&Lnopanic == 0 {
		l.Output(2, s)
	}
	panic(s)
}

func Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	if std.flag&Lnopanic == 0 {
		std.Output(2, s)
	}
	panic(s)
}

func (l *Logger) Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	if l.flag&Lnopanic == 0 {
		l.Output(2, s)
	}
	panic(s)
}

func Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	if std.flag&Lnopanic == 0 {
		std.Output(2, s)
	}
	panic(s)
}

func (l *Logger) Fatal(v ...interface{}) {
	if l.flag&Lnofatal == 0 {
		l.Output(2, fmt.Sprint(v...))
	}
	os.Exit(1)
}

func Fatal(v ...interface{}) {
	if std.flag&Lnofatal == 0 {
		std.Output(2, fmt.Sprint(v...))
	}
	os.Exit(1)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	if l.flag&Lnofatal == 0 {
		l.Output(2, fmt.Sprintf(format, v...))
	}
	os.Exit(1)
}

func Fatalf(format string, v ...interface{}) {
	if std.flag&Lnofatal == 0 {
		std.Output(2, fmt.Sprintf(format, v...))
	}
	os.Exit(1)
}

func (l *Logger) Fatalln(format string, v ...interface{}) {
	if l.flag&Lnofatal == 0 {
		l.Output(2, fmt.Sprintln(v...))
	}
	os.Exit(1)
}

func Fatalln(format string, v ...interface{}) {
	if std.flag&Lnofatal == 0 {
		std.Output(2, fmt.Sprintln(v...))
	}
	os.Exit(1)
}

func (l *Logger) Warn(v ...interface{}) {
	if l.flag&Lnowarn == 0 {
		l.Output(2, fmt.Sprint(v...))
	}
}

func Warn(v ...interface{}) {
	if std.flag&Lnowarn == 0 {
		std.Output(2, fmt.Sprint(v...))
	}
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	if l.flag&Lnowarn == 0 {
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func Warnf(format string, v ...interface{}) {
	if std.flag&Lnowarn == 0 {
		std.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Warnln(v ...interface{}) {
	if l.flag&Lnowarn == 0 {
		l.Output(2, fmt.Sprintln(v...))
	}
}

func Warnln(v ...interface{}) {
	if std.flag&Lnowarn == 0 {
		std.Output(2, fmt.Sprintln(v...))
	}
}

func (l *Logger) Print(v ...interface{}) {
	if l.flag&Lnoprint == 0 {
		l.Output(2, fmt.Sprint(v...))
	}
}

func Print(v ...interface{}) {
	if std.flag&Lnoprint == 0 {
		std.Output(2, fmt.Sprint(v...))
	}
}

func (l *Logger) Printf(format string, v ...interface{}) {
	if l.flag&Lnoprint == 0 {
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func Printf(format string, v ...interface{}) {
	if std.flag&Lnoprint == 0 {
		std.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Println(v ...interface{}) {
	if l.flag&Lnoprint == 0 {
		l.Output(2, fmt.Sprintln(v...))
	}
}

func Println(v ...interface{}) {
	if std.flag&Lnoprint == 0 {
		std.Output(2, fmt.Sprintln(v...))
	}
}

func (l *Logger) Debug(v ...interface{}) {
	if l.flag&Lnodebug == 0 {
		l.Output(2, fmt.Sprint(v...))
	}
}

func Debug(v ...interface{}) {
	if std.flag&Lnodebug == 0 {
		std.Output(2, fmt.Sprint(v...))
	}
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.flag&Lnodebug == 0 {
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func Debugf(format string, v ...interface{}) {
	if std.flag&Lnodebug == 0 {
		std.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Debugln(v ...interface{}) {
	if l.flag&Lnodebug == 0 {
		l.Output(2, fmt.Sprintln(v...))
	}
}

func Debugln(v ...interface{}) {
	if std.flag&Lnodebug == 0 {
		std.Output(2, fmt.Sprintln(v...))
	}
}

func (l *Logger) Flags() int {
	// access to an int int is always atomic
	return l.flag
}

func Flags() int {
	// access to an int int is always atomic
	return std.flag
}

func (l *Logger) SetFlags(flag int) {
	l.flag = flag
}

func SetFlags(flag int) {
	std.flag = flag
}

func (l *Logger) Prefix() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.prefix
}

func Prefix() string {
	return std.Prefix()
}

func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

func SetPrefix(prefix string) {
	std.SetPrefix(prefix)
}

func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}

func SetOutput(w io.Writer) {
	std.SetOutput(w)
}
