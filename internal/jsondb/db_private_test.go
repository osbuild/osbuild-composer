package jsondb

import (
	"errors"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomically(t *testing.T) {
	dir := t.TempDir()

	t.Run("success", func(t *testing.T) {
		octopus := []byte("🐙\n")

		// use an uncommon mode to check it's set correctly
		perm := os.FileMode(0750)

		err := writeFileAtomically(dir, "octopus", perm, func(f *os.File) error {
			_, err := f.Write(octopus)
			return err
		})
		require.NoError(t, err)

		// ensure that there are no stray temporary files
		infos, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Equal(t, 1, len(infos))
		require.Equal(t, "octopus", infos[0].Name())
		i, err := infos[0].Info()
		require.Nil(t, err)
		require.Equal(t, perm, i.Mode())

		filename := path.Join(dir, "octopus")
		contents, err := os.ReadFile(filename)
		require.NoError(t, err)
		require.Equal(t, octopus, contents)

		err = os.Remove(filename)
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		err := writeFileAtomically(dir, "no-octopus", 0750, func(f *os.File) error {
			return errors.New("something went wrong")
		})
		require.Error(t, err)

		_, err = os.Stat(path.Join(dir, "no-octopus"))
		require.Error(t, err)

		// ensure there are no stray temporary files
		infos, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Equal(t, 0, len(infos))
	})
}
