package jsondb_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jsondb"
)

type document struct {
	Animal  string `json:"animal"`
	CanSwim bool   `json:"can-swim"`
}

func cleanupTempDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	require.NoError(t, err)
}

// If the passed directory is not readable (writable), we should notice on the
// first read (write).
func TestDegenerate(t *testing.T) {
	t.Run("no-exist", func(t *testing.T) {
		db := jsondb.New("/non-existant-directory", 0755)

		var d document
		exist, err := db.Read("one", &d)
		assert.False(t, exist)
		assert.NoError(t, err)

		err = db.Write("one", &d)
		assert.Error(t, err)

		l, err := db.List()
		assert.Error(t, err)
		assert.Nil(t, l)
	})

	t.Run("invalid-json", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "jsondb-test-")
		require.NoError(t, err)
		defer cleanupTempDir(t, dir)

		db := jsondb.New(dir, 0755)

		// write-only file
		err = ioutil.WriteFile(path.Join(dir, "one.json"), []byte("{"), 600)
		require.NoError(t, err)

		var d document
		_, err = db.Read("one", &d)
		assert.Error(t, err)
	})
}

func TestCorrupt(t *testing.T) {
	dir, err := ioutil.TempDir("", "jsondb-test-")
	require.NoError(t, err)
	defer cleanupTempDir(t, dir)

	err = ioutil.WriteFile(path.Join(dir, "one.json"), []byte("{"), 0755)
	require.NoError(t, err)

	db := jsondb.New(dir, 0755)
	var d document
	_, err = db.Read("one", &d)
	require.Error(t, err)
}

func TestRead(t *testing.T) {
	dir, err := ioutil.TempDir("", "jsondb-test-")
	require.NoError(t, err)
	defer cleanupTempDir(t, dir)

	err = ioutil.WriteFile(path.Join(dir, "one.json"), []byte("true"), 0755)
	require.NoError(t, err)

	db := jsondb.New(dir, 0755)

	var b bool
	exists, err := db.Read("one", &b)
	require.NoError(t, err)
	require.True(t, exists)
	require.True(t, b)

	// nil means don't deserialize
	exists, err = db.Read("one", nil)
	require.NoError(t, err)
	require.True(t, exists)

	b = false
	exists, err = db.Read("two", &b)
	require.NoError(t, err)
	require.False(t, exists)
	require.False(t, b)

	// nil means don't deserialize
	exists, err = db.Read("two", nil)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestMultiple(t *testing.T) {
	dir, err := ioutil.TempDir("", "jsondb-test-")
	require.NoError(t, err)
	defer cleanupTempDir(t, dir)

	perm := os.FileMode(0600)
	documents := map[string]document{
		"one":   document{"octopus", true},
		"two":   document{"zebra", false},
		"three": document{"clownfish", true},
	}

	db := jsondb.New(dir, perm)

	for name, doc := range documents {
		err = db.Write(name, doc)
		require.NoError(t, err)
	}
	names, err := db.List()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"one", "two", "three"}, names)

	for name, doc := range documents {
		var d document
		exist, err := db.Read(name, &d)
		require.NoError(t, err)
		require.True(t, exist)
		require.Equalf(t, doc, d, "error retrieving document '%s'", name)
	}
}
