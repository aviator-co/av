package e2e_tests

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/shurcooL/githubv4"
)

// mockServerKey is used to store/retrieve the mock GitHub server from
// testscript's per-test value store.
type mockServerKey struct{}

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			return setupScriptRepo(env)
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"commit-file":             cmdCommitFile,
			"branch-parent":           cmdBranchParent,
			"branch-parent-hash":      cmdBranchParentHash,
			"branch-children":         cmdBranchChildren,
			"set-branch-pr":           cmdSetBranchPR,
			"set-branch-merge-commit": cmdSetBranchMergeCommit,
			"mock-pull":               cmdMockPull,
			"set-branch-prefix":       cmdSetBranchPrefix,
		},
	})
}

// setupScriptRepo creates an isolated git repository with av metadata,
// mirroring what gittest.NewTempRepo does for the traditional e2e tests.
// A mock GitHub GraphQL server is started for each test.
func setupScriptRepo(env *testscript.Env) error {
	// Create a wrapper script for av that redirects stdin from /dev/null.
	// testscript's exec provides stdin as a pipe. Any code in av that reads
	// stdin (lipgloss background detection, bubbletea prompts, interactive
	// input) can hang waiting on a pipe that never sends data. The original
	// e2e tests had cmd.Stdin=nil which maps to /dev/null (immediate EOF).
	wrapperDir := filepath.Join(env.WorkDir, "bin")
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		return err
	}
	wrapper := filepath.Join(wrapperDir, "av")
	if err := os.WriteFile(wrapper, []byte(fmt.Sprintf("#!/bin/sh\nexec %q \"$@\" </dev/null\n", avCmdPath)), 0o755); err != nil {
		return err
	}

	env.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+filepath.Dir(avCmdPath)+string(os.PathListSeparator)+os.Getenv("PATH"))
	env.Setenv("AV_GITHUB_TOKEN", "ghp_thisisntarealltokenitsjustfortesting")

	server := &mockGitHubServer{t: scriptLogger{env.T()}}
	server.Server = httptest.NewServer(server)
	env.Values[mockServerKey{}] = server
	env.Defer(server.Close)

	repoDir := filepath.Join(env.WorkDir, "repo")
	remoteDir := filepath.Join(env.WorkDir, "remote")
	for _, d := range []string{repoDir, remoteDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	if err := gitCmd(repoDir, "init", "--initial-branch=main"); err != nil {
		return err
	}
	if err := gitCmd(remoteDir, "init", "--bare"); err != nil {
		return err
	}
	if err := gitCmd(repoDir, "config", "user.name", "av-test"); err != nil {
		return err
	}
	if err := gitCmd(repoDir, "config", "user.email", "av-test@nonexistent"); err != nil {
		return err
	}
	if err := gitCmd(repoDir, "remote", "add", "origin", remoteDir, "--master=main"); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Hello World"), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"add", "README.md"},
		{"commit", "-m", "Initial commit"},
		{"push", "origin", "main"},
		{"symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"},
	} {
		if err := gitCmd(repoDir, args...); err != nil {
			return err
		}
	}

	avDir := filepath.Join(repoDir, ".git", "av")
	if err := os.MkdirAll(avDir, 0o755); err != nil {
		return err
	}
	db, _, err := jsonfiledb.OpenPath(filepath.Join(avDir, "av.db"))
	if err != nil {
		return err
	}
	tx := db.WriteTx()
	tx.SetRepository(meta.Repository{
		ID:    "R_nonexistent_",
		Owner: "aviator-co",
		Name:  "nonexistent",
	})
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := os.WriteFile(
		filepath.Join(avDir, "config.yml"),
		fmt.Appendf(nil, "github:\n    token: \"dummy_valid_token\"\n    baseUrl: %q\npullRequest:\n    branchNamePrefix: \"\"\n", server.URL),
		0o644,
	); err != nil {
		return err
	}

	env.Cd = repoDir
	return nil
}

func gitCmd(dir string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return nil
}

func resolveRef(dir, ref string) (string, error) {
	out, err := exec.CommandContext(context.Background(), "git", "-C", dir, "rev-parse", ref).Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func unescapeContent(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s
}

// commit-file [--amend] <filename> <content> [message]
//
// Writes content to filename, stages it, and commits. Content supports \n
// escape sequences. If message is omitted, defaults to "Write <filename>".
func cmdCommitFile(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("commit-file does not support negation")
	}
	amend := false
	var filtered []string
	for _, a := range args {
		if a == "--amend" {
			amend = true
		} else {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	if len(args) < 2 {
		ts.Fatalf("usage: commit-file [--amend] <filename> <content> [message]")
	}
	filename := args[0]
	content := unescapeContent(args[1])
	msg := fmt.Sprintf("Write %s", filename)
	if len(args) >= 3 {
		msg = args[2]
	}

	absPath := ts.MkAbs(filename)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		ts.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		ts.Fatalf("write file: %v", err)
	}

	dir := ts.MkAbs(".")
	if err := gitCmd(dir, "add", filename); err != nil {
		ts.Fatalf("%v", err)
	}
	commitArgs := []string{"commit", "-m", msg}
	if amend {
		if len(args) >= 3 {
			commitArgs = []string{"commit", "--amend", "-m", msg}
		} else {
			commitArgs = []string{"commit", "--amend", "--no-edit"}
		}
	}
	if err := gitCmd(dir, commitArgs...); err != nil {
		ts.Fatalf("%v", err)
	}
}

func openAvDB(ts *testscript.TestScript) *jsonfiledb.DB {
	gitDir := filepath.Join(ts.MkAbs("."), ".git")
	db, _, err := jsonfiledb.OpenPath(filepath.Join(gitDir, "av", "av.db"))
	if err != nil {
		ts.Fatalf("open av db: %v", err)
	}
	return db
}

// branch-parent <branch> <expected-parent>.
func cmdBranchParent(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: branch-parent <branch> <expected-parent>")
	}
	db := openAvDB(ts)
	br, ok := db.ReadTx().Branch(args[0])
	if !ok {
		ts.Fatalf("branch %q not found in database", args[0])
	}
	actual := br.Parent.Name
	if neg {
		if actual == args[1] {
			ts.Fatalf("branch %s: parent is %q (expected it not to be)", args[0], actual)
		}
	} else {
		if actual != args[1] {
			ts.Fatalf("branch %s: parent is %q, want %q", args[0], actual, args[1])
		}
	}
}

// branch-parent-hash <branch> <expected-ref>
//
// Asserts that the branching point commit hash matches the resolved ref.
func cmdBranchParentHash(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: branch-parent-hash <branch> <expected-ref>")
	}
	dir := ts.MkAbs(".")
	expectedHash, err := resolveRef(dir, args[1])
	if err != nil {
		ts.Fatalf("%v", err)
	}
	db := openAvDB(ts)
	br, ok := db.ReadTx().Branch(args[0])
	if !ok {
		ts.Fatalf("branch %q not found in database", args[0])
	}
	actual := br.Parent.BranchingPointCommitHash
	if neg {
		if actual == expectedHash {
			ts.Fatalf("branch %s: parent hash is %s (expected different)", args[0], actual)
		}
	} else {
		if actual != expectedHash {
			ts.Fatalf("branch %s: parent hash is %q, want %q", args[0], actual, expectedHash)
		}
	}
}

// branch-children <branch> [expected-children...].
func cmdBranchChildren(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 1 {
		ts.Fatalf("usage: branch-children <branch> [expected-children...]")
	}
	db := openAvDB(ts)
	children := meta.ChildrenNames(db.ReadTx(), args[0])
	expected := args[1:]
	sort.Strings(children)
	sort.Strings(expected)
	match := slices.Equal(children, expected)
	if neg && match {
		ts.Fatalf("branch %s: children are %v (expected different)", args[0], children)
	}
	if !neg && !match {
		ts.Fatalf("branch %s: children are %v, want %v", args[0], children, expected)
	}
}

// set-branch-pr <branch> <id> <number> <state>
//
// Sets PullRequest metadata on a branch in the av database.
func cmdSetBranchPR(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("set-branch-pr does not support negation")
	}
	if len(args) != 4 {
		ts.Fatalf("usage: set-branch-pr <branch> <id> <number> <state>")
	}
	number, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		ts.Fatalf("invalid number: %v", err)
	}
	db := openAvDB(ts)
	tx := db.WriteTx()
	br, ok := tx.Branch(args[0])
	if !ok {
		ts.Fatalf("branch %q not found in database", args[0])
	}
	br.PullRequest = &meta.PullRequest{ID: args[1], Number: number, State: githubv4.PullRequestState(args[3])}
	tx.SetBranch(br)
	if err := tx.Commit(); err != nil {
		ts.Fatalf("commit: %v", err)
	}
}

// set-branch-merge-commit <branch> <ref>
//
// Sets the MergeCommit field on a branch. <ref> is resolved via git rev-parse.
func cmdSetBranchMergeCommit(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("set-branch-merge-commit does not support negation")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: set-branch-merge-commit <branch> <ref>")
	}
	dir := ts.MkAbs(".")
	oid, err := resolveRef(dir, args[1])
	if err != nil {
		ts.Fatalf("%v", err)
	}
	db := openAvDB(ts)
	tx := db.WriteTx()
	br, ok := tx.Branch(args[0])
	if !ok {
		ts.Fatalf("branch %q not found in database", args[0])
	}
	br.MergeCommit = oid
	tx.SetBranch(br)
	if err := tx.Commit(); err != nil {
		ts.Fatalf("commit: %v", err)
	}
}

// mock-pull <headRefName> <number> <state> [mergeCommitRef]
//
// Adds a mock PR to the GitHub server. If mergeCommitRef is provided,
// it is resolved via git rev-parse.
func cmdMockPull(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("mock-pull does not support negation")
	}
	if len(args) < 3 {
		ts.Fatalf("usage: mock-pull <headRefName> <number> <state> [mergeCommitRef]")
	}
	number, err := strconv.Atoi(args[1])
	if err != nil {
		ts.Fatalf("invalid number: %v", err)
	}
	pr := mockPR{
		ID:          fmt.Sprintf("nodeid-%d", number),
		Number:      number,
		HeadRefName: args[0],
		State:       args[2],
	}
	if len(args) >= 4 && args[3] != "" {
		dir := ts.MkAbs(".")
		oid, err := resolveRef(dir, args[3])
		if err != nil {
			ts.Fatalf("%v", err)
		}
		pr.MergeCommitOID = oid
	}
	server := ts.Value(mockServerKey{}).(*mockGitHubServer)
	server.pulls = append(server.pulls, pr)
}

// set-branch-prefix <prefix>
//
// Updates the branchNamePrefix in the av config.
func cmdSetBranchPrefix(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("set-branch-prefix does not support negation")
	}
	prefix := ""
	if len(args) > 0 {
		prefix = args[0]
	}
	configPath := filepath.Join(ts.MkAbs("."), ".git", "av", "config.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		ts.Fatalf("read config: %v", err)
	}
	newContent := strings.Replace(string(content),
		`branchNamePrefix: ""`,
		fmt.Sprintf("branchNamePrefix: %q", prefix), 1)
	if err := os.WriteFile(configPath, []byte(newContent), 0o644); err != nil {
		ts.Fatalf("write config: %v", err)
	}
}

// scriptLogger adapts testscript.T (which has Log) to mockLogger (which needs Logf).
type scriptLogger struct {
	t testscript.T
}

func (l scriptLogger) Logf(format string, args ...any) {
	l.t.Log(fmt.Sprintf(format, args...))
}
