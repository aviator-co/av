package cleanup_test

import (
	"testing"

	"github.com/aviator-co/av/internal/utils/cleanup"
)

func TestCleanup(t *testing.T) {
	var cu cleanup.Cleanup
	flag1 := false
	flag2 := false
	cu.Add(func() {
		if !flag2 {
			t.Error("cleanup functions should run in reverse order")
		}
		flag1 = true
	})
	cu.Add(func() {
		if flag1 {
			t.Error("cleanup functions should run in reverse order")
		}
		flag2 = true
	})
	cu.Cleanup()
}

func TestCleanupCancel(t *testing.T) {
	var cu cleanup.Cleanup
	cu.Add(func() {
		t.Error("cleanup shouldn't run")
	})
	cu.Cancel()
	cu.Cleanup()
}

func TestCleanupEmpty(t *testing.T) {
	var cu cleanup.Cleanup
	cu.Cleanup()
}
