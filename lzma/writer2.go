package lzma

import (
	"errors"
	"runtime"

	"github.com/ulikunitz/lz"
)

type Writer2Config struct {
	LZCfg          lz.Configurator
	Properties     Properties
	ZeroProperties bool
	Worker         int
}

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
	if cfg.Worker < 0 {
		return errors.New("lzma: Worker must be larger than 0")
	}

	return nil
}

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

	if cfg.Worker == 0 {
		cfg.Worker = runtime.NumCPU()
	}
}

// TODO: type Writer2
