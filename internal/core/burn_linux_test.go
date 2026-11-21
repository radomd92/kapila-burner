//go:build linux

package core

import "testing"

func TestSafeWriterLock(t *testing.T) {
	a, err := writerLock("/dev/mock-core-test")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if b, err := writerLock("/dev/mock-core-test"); err == nil {
		b.Close()
		t.Fatal("writer not exclusive")
	}
}
