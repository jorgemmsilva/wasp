package main

import (
	"os"

	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

type soloContext struct {
	cleanup []func()
}

func (s *soloContext) cleanupAll() {
	for i := len(s.cleanup) - 1; i >= 0; i-- {
		s.cleanup[i]()
	}
}

func (s *soloContext) Cleanup(f func()) {
	s.cleanup = append(s.cleanup, f)
}

func (*soloContext) Errorf(format string, args ...interface{}) {
	log.Printf("error: "+format, args)
}

func (*soloContext) FailNow() {
	os.Exit(1)
}

func (s *soloContext) Fatalf(format string, args ...any) {
	log.Printf("fatal: "+format, args)
	s.FailNow()
}

func (*soloContext) Helper() {
}

func (*soloContext) Logf(format string, args ...any) {
	log.Printf(format, args...)
}

func (*soloContext) Name() string {
	return "solo"
}
