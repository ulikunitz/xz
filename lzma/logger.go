package lzma

type logger interface {
	Logf(format string, args ...interface{})
}

type zeroLogger struct{}

func (z zeroLogger) Logf(format string, args ...interface{}) {}
