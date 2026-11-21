//go:build !linux

package core

import (
	"context"
	"strings"
	"testing"
)

func TestUnsupportedBurn(t *testing.T) {
	s := testStudio(t)
	recorders, err := s.Recorders(context.Background())
	if recorders == nil || len(recorders) != 0 || err == nil || !strings.Contains(err.Error(), "external burning software") {
		t.Fatalf("recorders: %v, %v", recorders, err)
	}
	if job, err := s.Burn(context.Background(), BurnRequest{}); job != nil || err == nil || !strings.Contains(err.Error(), "only on Linux") {
		t.Fatalf("burn: %v, %v", job, err)
	}
	if s.Snapshot() != nil {
		t.Fatal("unsupported burn published a job")
	}
}
