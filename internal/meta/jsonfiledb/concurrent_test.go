package jsonfiledb_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/require"
)

// Two DB handles on one file stand in for two av processes (for example two
// git worktrees, which share av.db). Issue 789: the second commit wrote back
// a snapshot taken before the first commit and erased its branch.
func TestCommitKeepsWritesFromAnotherHandle(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "av.db")

	db1, _, err := jsonfiledb.OpenPath(fp)
	require.NoError(t, err)
	db2, _, err := jsonfiledb.OpenPath(fp)
	require.NoError(t, err)

	tx2 := db2.WriteTx() // snapshot taken before db1 commits
	tx1 := db1.WriteTx()
	tx1.SetBranch(meta.Branch{Name: "adopted", Parent: meta.BranchState{Name: "main", Trunk: true}})
	require.NoError(t, tx1.Commit())

	tx2.SetBranch(meta.Branch{Name: "committed", Parent: meta.BranchState{Name: "main", Trunk: true}})
	require.NoError(t, tx2.Commit())

	db3, _, err := jsonfiledb.OpenPath(fp)
	require.NoError(t, err)
	_, ok := db3.ReadTx().Branch("adopted")
	require.True(t, ok, "branch written by the other handle must survive")
	_, ok = db3.ReadTx().Branch("committed")
	require.True(t, ok, "branch written by this handle must be saved")

	// The handle that committed last also sees the other handle's branch.
	_, ok = db2.ReadTx().Branch("adopted")
	require.True(t, ok)
}

func TestCommitAppliesDeletesOnFreshState(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "av.db")
	db1, _, err := jsonfiledb.OpenPath(fp)
	require.NoError(t, err)
	tx := db1.WriteTx()
	tx.SetBranch(meta.Branch{Name: "old"})
	require.NoError(t, tx.Commit())

	db2, _, err := jsonfiledb.OpenPath(fp)
	require.NoError(t, err)
	tx2 := db2.WriteTx()

	tx1 := db1.WriteTx()
	tx1.SetBranch(meta.Branch{Name: "new"})
	require.NoError(t, tx1.Commit())

	tx2.DeleteBranch("old")
	require.NoError(t, tx2.Commit())

	db3, _, err := jsonfiledb.OpenPath(fp)
	require.NoError(t, err)
	_, ok := db3.ReadTx().Branch("old")
	require.False(t, ok, "delete must be applied")
	_, ok = db3.ReadTx().Branch("new")
	require.True(t, ok, "branch added by the other handle must survive the delete")
}

func TestConcurrentHandlesLoseNoWrites(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "av.db")
	const handles, perHandle = 8, 25
	var wg sync.WaitGroup
	for h := range handles {
		db, _, err := jsonfiledb.OpenPath(fp)
		require.NoError(t, err)
		wg.Add(1)
		go func(h int, db *jsonfiledb.DB) {
			defer wg.Done()
			for i := range perHandle {
				tx := db.WriteTx()
				tx.SetBranch(meta.Branch{Name: fmt.Sprintf("b-%d-%d", h, i)})
				if err := tx.Commit(); err != nil {
					t.Error(err)
					return
				}
			}
		}(h, db)
	}
	wg.Wait()

	db, _, err := jsonfiledb.OpenPath(fp)
	require.NoError(t, err)
	require.Len(t, db.ReadTx().AllBranches(), handles*perHandle)

	// The file is always complete JSON, and no temporary file is left behind.
	data, err := os.ReadFile(fp)
	require.NoError(t, err)
	require.True(t, json.Valid(data))
	entries, err := os.ReadDir(filepath.Dir(fp))
	require.NoError(t, err)
	for _, e := range entries {
		require.Contains(t, []string{"av.db", "av.db.lock"}, e.Name())
	}
}
