package e2e_tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"emperror.dev/errors"
	"github.com/kr/text"
	"github.com/stretchr/testify/require"
)

var avCmdPath string

func init() {
	if err := os.Setenv("AV_GITHUB_TOKEN", "ghp_thisisntarealltokenitsjustfortesting"); err != nil {
		panic(err)
	}

	cmd := exec.Command("go", "build", "../cmd/av")
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

type AvOutput struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

func Cmd(t *testing.T, exe string, args ...string) AvOutput {
	cmd := exec.Command(exe, args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		t.Fatal(err)
	}

	output := AvOutput{
		ExitCode: cmd.ProcessState.ExitCode(),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	t.Logf("Running av-cli\n"+
		"args: %v\n"+
		"exit code: %v\n"+
		"stdout:\n"+
		"%s"+
		"stderr:\n"+
		"%s",
		args,
		cmd.ProcessState.ExitCode(),
		text.Indent(stdout.String(), "  "),
		text.Indent(stderr.String(), "  "),
	)
	return output
}

func RequireCmd(t *testing.T, exe string, args ...string) AvOutput {
	output := Cmd(t, exe, args...)
	require.Equal(t, 0, output.ExitCode, "cmd %s: exited with %v", args, output.ExitCode)
	return output
}

func Av(t *testing.T, args ...string) AvOutput {
	args = append([]string{"--debug"}, args...)
	return Cmd(t, avCmdPath, args...)
}

func RequireAv(t *testing.T, args ...string) AvOutput {
	t.Helper()
	output := Av(t, args...)
	require.Equal(t, 0, output.ExitCode, "av %s: exited with %v", args, output.ExitCode)
	return output
}

func Chdir(t *testing.T, dir string) {
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(current); err != nil {
			t.Fatal(err)
		}
	})
}
