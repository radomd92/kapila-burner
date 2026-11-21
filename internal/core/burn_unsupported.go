//go:build !linux

package core

import "context"

const unsupportedBurn = "Physical VCD burning is supported only on Linux; use external burning software to burn the authored CUE/BIN image on this platform."

func (s *Engine) Recorders(ctx context.Context) ([]Recorder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []Recorder{}, errorf(ctx, unsupportedBurn)
}

func (s *Engine) Burn(ctx context.Context, request BurnRequest) (*Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errorf(ctx, unsupportedBurn)
}
