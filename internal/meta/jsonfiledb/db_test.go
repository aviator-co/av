package jsonfiledb_test

import (
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestJSONFileDB(t *testing.T) {
	tempfile := t.TempDir() + "/db.json"

	db, err := jsonfiledb.Open(tempfile)
	require.NoError(t, err, "db open should succeed if state file does not exist")

	if _, ok := db.Branch("foo"); ok {
		t.Error("non existent branch should not be found")
	}

	err = db.WithTx(func(tx meta.WriteTx) {
		tx.SetBranch(meta.Branch{Name: "foo"})
	})
	require.NoError(t, err, "tx commit should succeed")

	err = db.WithTx(func(tx meta.WriteTx) {
		tx.SetBranch(meta.Branch{Name: "bar"})
		tx.Abort()
	})
	require.NoError(t, err, "aborted tx should not return error")
	if _, ok := db.Branch("bar"); ok {
		t.Error("aborted tx should not commit changes")
	}

	foo, ok := db.Branch("foo")
	require.True(t, ok, "branch should be found")
	require.Equal(t, "foo", foo.Name, "branch name should match")

	// Re-open the database and cause it to re-read from disk
	db, err = jsonfiledb.Open(tempfile)
	require.NoError(t, err, "db open should succeed if state file exists")
	foo, ok = db.Branch("foo")
	require.True(t, ok, "branch should be found after re-open")
	require.Equal(t, "foo", foo.Name, "branch name should match")
}
