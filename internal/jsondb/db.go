// Package jsondb implements a simple database of JSON documents, backed by the
// file system.
//
// It supports two operations: Read() and Write(). Their signatures mirror
// those of json.Unmarshal() and json.Marshal():
//
//     err := db.Write("my-string", "octopus")
//
//     var v string
//     exists, err := db.Read("my-string", &v)
//
// The JSON documents are stored in a directory, in the form name.json (name as
// passed to Read() and Write()). Thus, names may only contain characters that
// may appear in filenames.

package jsondb

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type JSONDatabase struct {
	dir  string
	perm os.FileMode
}

// Create a new JSONDatabase in `dir`. Each document that is saved to it will
// have a file mode of `perm`.
func New(dir string, perm os.FileMode) *JSONDatabase {
	return &JSONDatabase{dir, perm}
}

// Reads the value at `name`. `document` must be a type that is deserializable
// from the JSON document `name`, or nil to not deserialize at all. Returns
// false if a document with `name` does not exist.
func (db *JSONDatabase) Read(name string, document interface{}) (bool, error) {
	f, err := os.Open(path.Join(db.dir, name+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("error accessing db file %s: %v", name, err)
	}
	defer f.Close()

	if document != nil {
		err = json.NewDecoder(f).Decode(&document)
		if err != nil {
			return false, fmt.Errorf("error reading db file %s: %v", name, err)
		}
	}

	return true, nil
}

// Returns a list of all documents' names.
func (db *JSONDatabase) List() ([]string, error) {
	f, err := os.Open(db.dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dirNames, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(dirNames))
	for i, name := range dirNames {
		names[i] = strings.TrimSuffix(name, ".json")
	}

	return names, nil
}

// Delete will delete the file from the database
func (db *JSONDatabase) Delete(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("missing jsondb document name")
	}
	return os.Remove(filepath.Join(db.dir, name+".json"))
}

// Writes `document` to `name`, overwriting a previous document if it exists.
// `document` must be serializable to JSON.
func (db *JSONDatabase) Write(name string, document interface{}) error {
	return writeFileAtomically(db.dir, name+".json", db.perm, func(f *os.File) error {
		return json.NewEncoder(f).Encode(document)
	})
}

// writeFileAtomically writes data to `filename` in `directory` atomically, by
// first creating a temporary file in `directory` and only moving it when
// writing succeeded. `writer` gets passed the open file handle to write to and
// does not need to take care of closing it.
func writeFileAtomically(dir, filename string, mode os.FileMode, writer func(f *os.File) error) error {
	tmpfile, err := os.CreateTemp(dir, filename+"-*.tmp")
	if err != nil {
		return err
	}

	// Remove `tmpfile` in each error case. We cannot use `defer` here,
	// because `tmpfile` shouldn't be removed when everything works: it
	// will be renamed to `filename`. Ignore errors from `os.Remove()`,
	// because the error relating to `tempfile` is more relevant.

	err = tmpfile.Chmod(mode)
	if err != nil {
		_ = os.Remove(tmpfile.Name())
		return fmt.Errorf("error setting permissions on %s: %v", tmpfile.Name(), err)
	}

	err = writer(tmpfile)
	if err != nil {
		_ = os.Remove(tmpfile.Name())
		return fmt.Errorf("error writing to %s: %v", tmpfile.Name(), err)
	}

	err = tmpfile.Close()
	if err != nil {
		_ = os.Remove(tmpfile.Name())
		return fmt.Errorf("error closing %s: %v", tmpfile.Name(), err)
	}

	err = os.Rename(tmpfile.Name(), path.Join(dir, filename))
	if err != nil {
		_ = os.Remove(tmpfile.Name())
		return fmt.Errorf("error moving %s to %s: %v", filepath.Base(tmpfile.Name()), filename, err)
	}

	return nil
}
