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

// If the passed directory is not readable (writable), we should notice on the
// first read (write).
func TestDegenerate(t *testing.T) {
	db := jsondb.New("/non-existant-directory", 0755)

	var d document
	exist, err := db.Read("one", &d)
	assert.False(t, exist)
	assert.NoError(t, err)

	err = db.Write("one", &d)
	assert.Error(t, err)
}

func TestCorrupt(t *testing.T) {
	dir, err := ioutil.TempDir("", "jsondb-test-")
	require.NoError(t, err)

	defer func() {
		err := os.RemoveAll(dir)
		require.NoError(t, err)
	}()

	err = ioutil.WriteFile(path.Join(dir, "one.json"), []byte("{"), 0755)
	require.NoError(t, err)

	db := jsondb.New(dir, 0755)

	var d document
	_, err = db.Read("one", &d)
	require.Error(t, err)
}

func TestMultiple(t *testing.T) {
	dir, err := ioutil.TempDir("", "jsondb-test-")
	require.NoError(t, err)

	defer func() {
		err := os.RemoveAll(dir)
		require.NoError(t, err)
	}()

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
	infos, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	require.Equal(t, len(infos), len(documents))
	for _, info := range infos {
		require.Equal(t, perm, info.Mode())
	}

	for name, doc := range documents {
		var d document
		exist, err := db.Read(name, &d)
		require.NoError(t, err)
		require.True(t, exist)
		require.Equalf(t, doc, d, "error retrieving document '%s'", name)
	}
}
