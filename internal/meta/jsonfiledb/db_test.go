package jsonfiledb_test

import (
	"testing"

	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONFileDB(t *testing.T) {
	tempfile := t.TempDir() + "/db.json"

	db, err := jsonfiledb.OpenPath(tempfile)
	require.NoError(t, err, "db open should succeed if state file does not exist")

	if _, ok := db.ReadTx().Branch("foo"); ok {
		t.Error("non existent branch should not be found")
	}

	tx := db.WriteTx()
	tx.SetBranch(meta.Branch{Name: "foo"})
	require.NoError(t, tx.Commit(), "tx commit should succeed")

	tx = db.WriteTx()
	tx.SetBranch(meta.Branch{Name: "bar"})
	bar, ok := tx.Branch("bar")
	if !ok {
		t.Error("modifications should be visible within a transaction")
	}
	assert.Equal(t, "bar", bar.Name, "branch name should match")
	tx.Abort()

	if _, ok := db.ReadTx().Branch("bar"); ok {
		t.Error("aborted tx should not commit changes")
	}

	foo, ok := db.ReadTx().Branch("foo")
	require.True(t, ok, "branch should be found")
	require.Equal(t, "foo", foo.Name, "branch name should match")

	// Re-open the database and cause it to re-read from disk
	db, err = jsonfiledb.OpenPath(tempfile)
	require.NoError(t, err, "db open should succeed if state file exists")
	foo, ok = db.ReadTx().Branch("foo")
	require.True(t, ok, "branch should be found after re-open")
	require.Equal(t, "foo", foo.Name, "branch name should match")
}
