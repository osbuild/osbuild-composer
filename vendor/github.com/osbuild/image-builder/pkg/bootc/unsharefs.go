package bootc

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const fieldSep = "\x1f"

// podmanUnshareFS is an fs.FS that accesses a container or image
// mount that exists inside the rootless "podman unshare" mount
// namespace.
type podmanUnshareFS struct {
	root      string
	argPrefix []string
}

// Ensure we implement the related ifaces
var _ fs.FS = podmanUnshareFS{}
var _ fs.ReadFileFS = podmanUnshareFS{}
var _ fs.StatFS = podmanUnshareFS{}
var _ fs.ReadDirFS = podmanUnshareFS{}

func newPodmanUnshareFS(root string) podmanUnshareFS {
	return podmanUnshareFS{root: root, argPrefix: []string{"podman", "unshare"}}
}

func (fsys podmanUnshareFS) run(args ...string) ([]byte, error) {
	argv := append(append([]string{}, fsys.argPrefix...), args...)
	/* #nosec G204 */
	cmd := exec.Command(argv[0], argv[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w\nstderr:\n%s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func (fsys podmanUnshareFS) fullPath(name string) string {
	return filepath.Join(fsys.root, name)
}

func (fsys podmanUnshareFS) exists(name string) bool {
	_, err := fsys.run("test", "-e", fsys.fullPath(name))
	return err == nil
}

// Ensure os.IsNotExist() works on the errors
func (fsys podmanUnshareFS) wrapErr(op, name string, err error) error {
	if !fsys.exists(name) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}
	return &fs.PathError{Op: op, Path: name, Err: err}
}

func (fsys podmanUnshareFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	out, err := fsys.run("cat", fsys.fullPath(name))
	if err != nil {
		return nil, fsys.wrapErr("open", name, err)
	}
	return out, nil
}

// Internal stat that doesn't validate path
func (fsys podmanUnshareFS) stat(op, name string) (fs.FileInfo, error) {
	// -L => follows symlinks
	out, err := fsys.run("stat", "-L", "-c", "%s"+fieldSep+"%F", fsys.fullPath(name))
	if err != nil {
		return nil, fsys.wrapErr(op, name, err)
	}
	parts := strings.SplitN(strings.TrimRight(string(out), "\n"), fieldSep, 2)
	size, _ := strconv.ParseInt(parts[0], 10, 64)
	isDir := len(parts) > 1 && parts[1] == "directory"
	return fileInfo{name: path.Base(name), size: size, isDir: isDir}, nil
}

func (fsys podmanUnshareFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	return fsys.stat("stat", name)
}

func (fsys podmanUnshareFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	// -L follows symlinks so that symlinked directories report as 'd'.
	out, err := fsys.run("find", "-L", fsys.fullPath(name),
		"-maxdepth", "1", "-mindepth", "1", "-printf", "%y"+fieldSep+"%s"+fieldSep+`%f\0`)
	if err != nil {
		return nil, fsys.wrapErr("open", name, err)
	}

	var entries []fs.DirEntry
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		f := strings.SplitN(string(rec), fieldSep, 3)
		if len(f) != 3 {
			continue
		}
		size, _ := strconv.ParseInt(f[1], 10, 64)
		entries = append(entries, dirEntry{name: f[2], isDir: f[0] == "d", size: size})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (fsys podmanUnshareFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	fi, err := fsys.stat("open", name)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return &dirFile{fsys: fsys, name: name, info: fi}, nil
	}
	data, err := fsys.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return &memFile{
		reader: bytes.NewReader(data),
		info:   fileInfo{name: path.Base(name), size: int64(len(data))},
	}, nil
}

type fileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi fileInfo) Name() string { return fi.name }
func (fi fileInfo) Size() int64  { return fi.size }
func (fi fileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0555
	}
	return 0444
}
func (fi fileInfo) ModTime() time.Time { return time.Time{} }
func (fi fileInfo) IsDir() bool        { return fi.isDir }
func (fi fileInfo) Sys() any           { return nil }

type dirEntry struct {
	name  string
	isDir bool
	size  int64
}

func (e dirEntry) Name() string { return e.name }
func (e dirEntry) IsDir() bool  { return e.isDir }
func (e dirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}
func (e dirEntry) Info() (fs.FileInfo, error) {
	return fileInfo{name: e.name, isDir: e.isDir, size: e.size}, nil
}

type memFile struct {
	reader *bytes.Reader
	info   fs.FileInfo
}

func (f *memFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *memFile) Read(p []byte) (int, error) { return f.reader.Read(p) }
func (f *memFile) Close() error               { return nil }

type dirFile struct {
	fsys    podmanUnshareFS
	name    string
	info    fs.FileInfo
	entries []fs.DirEntry
	offset  int
}

// Ensure we implement the directory file iface
var _ fs.ReadDirFile = (*dirFile)(nil)

func (d *dirFile) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *dirFile) Close() error               { return nil }

// Read on a directory is not supported, matching *os.File behaviour.
func (d *dirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: fs.ErrInvalid}
}

func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.entries == nil {
		entries, err := d.fsys.ReadDir(d.name)
		if err != nil {
			return nil, err
		}
		// Use a non-nil slice so a subsequent call is not treated as
		// uninitialised even when the directory is empty.
		if entries == nil {
			entries = []fs.DirEntry{}
		}
		d.entries = entries
	}

	if n <= 0 {
		rest := d.entries[d.offset:]
		d.offset = len(d.entries)
		return rest, nil
	}

	if d.offset >= len(d.entries) {
		return nil, io.EOF
	}
	end := d.offset + n
	if end > len(d.entries) {
		end = len(d.entries)
	}
	batch := d.entries[d.offset:end]
	d.offset = end
	return batch, nil
}
