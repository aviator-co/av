package e2e_tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

var avCmdPath string

func init() {
	cmd := exec.CommandContext(context.Background(), "go", "build", "../cmd/av")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic(err)
	}
	var err error
	avCmdPath, err = filepath.Abs("./av")
	if err != nil {
		panic(err)
	}
}
