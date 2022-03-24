package lzma

import (
	"errors"
	"io"
	"runtime"

	"github.com/ulikunitz/lz"
)

// Writer2Config provides the configuration parameters for an LZMA2 writer.
type Writer2Config struct {
	LZCfg          lz.Configurator
	Properties     Properties
	ZeroProperties bool
	Workers        int
}

// Verify checks whether the configuration is consistent and correct. Usually
// call ApplyDefaults before this method.
func (cfg *Writer2Config) Verify() error {
	if cfg == nil {
		return errors.New("lzma: Writer2Config pointer must not be nil")
	}

	var err error
	type verifier interface {
		Verify() error
	}
	v, ok := cfg.LZCfg.(verifier)
	if ok {
		if err = v.Verify(); err != nil {
			return err
		}
	}

	if err = cfg.Properties.Verify(); err != nil {
		return err
	}
	if cfg.Workers < 0 {
		return errors.New("lzma: Worker must be larger than 0")
	}

	return nil
}

// ApplyDefaults replaces zero values with default values.
func (cfg *Writer2Config) ApplyDefaults() {
	if cfg.LZCfg == nil {
		cfg.LZCfg = &lz.Config{}
	}

	type ad interface {
		ApplyDefaults()
	}
	if a, ok := cfg.LZCfg.(ad); ok {
		a.ApplyDefaults()
	}

	var zeroProps = Properties{}
	if cfg.Properties == zeroProps && !cfg.ZeroProperties {
		cfg.Properties = Properties{3, 0, 2}
	}

	if cfg.Workers == 0 {
		cfg.Workers = runtime.NumCPU()
	}
}

// Writer2 is an interface that can Write, Close and Flush.
type Writer2 interface {
	io.WriteCloser
	Flush() error
}

// NewWriter2 generates an LZMA2 writer for the default configuration.
func NewWriter2(z io.Writer) (w Writer2, err error) {
	return NewWriter2Config(z, Writer2Config{})
}

// NewWriter2Config constrcuts an LZMA2 writer for a specific configuration.
// Note taht the implementation for cfg.Workers > 2 uses go routines.
func NewWriter2Config(z io.Writer, cfg Writer2Config) (w Writer2, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}

	if cfg.Workers == 1 {
		seq, err := cfg.LZCfg.NewSequencer()
		if err != nil {
			return nil, err
		}
		var cw chunkWriter
		if err = cw.init(z, seq, nil, cfg.Properties); err != nil {
			return nil, err
		}
		return &cw, nil
	}

	panic("TODO")
}
