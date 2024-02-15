package util

import (
	"context"
	"errors"
	"time"
)

const (
	defaultWaitInterval = 1 * time.Second
	defaultTimeoutMsg   = "wating timed out"
)

type WaitAction bool

const (
	WaitActionDone        = WaitAction(true)
	WaitActionKeepWaiting = WaitAction(false)
)

type WaitOpts struct {
	RetryInterval time.Duration
	TimeoutMsg    string
}

func WaitUntil(ctx context.Context, f func() (WaitAction, error), waitOpts ...WaitOpts) error {
	opts := WaitOpts{
		RetryInterval: defaultWaitInterval,
		TimeoutMsg:    defaultTimeoutMsg,
	}
	if len(waitOpts) > 0 {
		if waitOpts[0].RetryInterval != 0 {
			opts.RetryInterval = waitOpts[0].RetryInterval
		}
		if waitOpts[0].TimeoutMsg != "" {
			opts.TimeoutMsg = waitOpts[0].TimeoutMsg
		}
	}
	for {
		select {
		case <-ctx.Done():
			return errors.New(opts.TimeoutMsg)
		case <-time.After(opts.RetryInterval):
			action, err := f()
			if action == WaitActionDone {
				return err
			}
		}
	}
}
