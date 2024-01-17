// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package testlogger

import (
	"io"
	"time"

	"github.com/iotaledger/hive.go/log"
	"github.com/iotaledger/hive.go/runtime/options"
)

type TestingT interface { // Interface so there's no need to pass the concrete type
	Name() string
}

// NewSimple produces a logger adjusted for test cases.
func NewSimple(debug bool, opts ...options.Option[log.Options]) log.Logger {
	level := "info"
	if debug {
		level = "debug"
	}
	var err error
	loggerLevel, err := log.LevelFromString(level)
	if err != nil {
		panic(err)
	}
	return log.NewLogger(append([]options.Option[log.Options]{
		log.WithLevel(loggerLevel),
		log.WithTimeFormat(time.RFC3339),
	}, opts...)...)
}

// NewLogger produces a logger adjusted for test cases.
func NewLogger(t TestingT, opts ...options.Option[log.Options]) log.Logger {
	return NewSimple(true, append([]options.Option[log.Options]{
		log.WithName(t.Name()),
	}, opts...)...)
}

func NewSilentLogger(debug bool, name string) log.Logger {
	return NewSimple(debug, log.WithName(name), log.WithOutput(io.Discard))
}

// WithLevel returns a logger with a level increased.
// Can be useful in tests to disable logging in some parts of the system.
func WithLevel(log log.Logger, level log.Level) log.Logger {
	log.SetLogLevel(level)
	return log
}
