package tools

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse"
)

type FileSafeWriter struct {
	*ledger.File // the root file; nil until first Add
	dir     string
	entries []fileEntry
}

type fileEntry struct {
	path string
	orig []byte
	file *ledger.File
}

// NewFileSafeWriter parses the ledger file at rootPath and returns a writer
// rooted at the file's directory.
func NewFileSafeWriter(rootPath string) (*FileSafeWriter, error) {
	dir := filepath.Dir(rootPath)
	base := filepath.Base(rootPath)

	data, err := os.ReadFile(rootPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", rootPath, err)
	}
	f, err := parse.ParseLedgerString(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", rootPath, err)
	}

	return &FileSafeWriter{
		File: f,
		dir:  dir,
		entries: []fileEntry{{
			path: base,
			orig: data,
			file: f,
		}},
	}, nil
}

// Add reads the file at path from disk, parses it as a ledger file, and adds it
// to the writer. Returns the parsed file. The signature matches the callback
// expected by ledger.File.Includes.
func (w *FileSafeWriter) Add(path string) (*ledger.File, error) {
	data, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := parse.ParseLedgerString(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	w.entries = append(w.entries, fileEntry{path: path, orig: data, file: f})
	return f, nil
}

// Commit formats all loaded files, compares against the originals, and if any
// file changed, backs up all originals to a timestamped tar.gz and writes the
// formatted versions to disk.
func (w *FileSafeWriter) Commit() error {
	type formatted struct {
		path    string
		orig    []byte
		new     []byte
		changed bool
	}
	var fmts []formatted
	anyChanged := false

	for _, e := range w.entries {
		var buf bytes.Buffer
		if err := e.file.Format(&buf); err != nil {
			return fmt.Errorf("formatting %s: %w", e.path, err)
		}
		fmtd := buf.Bytes()
		changed := sha256.Sum256(e.orig) != sha256.Sum256(fmtd)
		if changed {
			anyChanged = true
		}
		fmts = append(fmts, formatted{e.path, e.orig, fmtd, changed})
	}

	if !anyChanged {
		return nil
	}

	// Backup all originals
	backupName := filepath.Join(w.dir, "backup-"+time.Now().Format("20060102-150405")+".tar.gz")
	backup, err := os.Create(backupName)
	if err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}
	defer backup.Close()

	gw := gzip.NewWriter(backup)
	tw := tar.NewWriter(gw)

	for _, f := range fmts {
		hdr := &tar.Header{
			Name:    filepath.Base(f.path),
			Size:    int64(len(f.orig)),
			Mode:    0644,
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("writing tar header: %w", err)
		}
		if _, err := tw.Write(f.orig); err != nil {
			return fmt.Errorf("writing tar entry: %w", err)
		}
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("closing gzip: %w", err)
	}
	backup.Close()

	// Write all formatted versions
	for _, f := range fmts {
		fullPath := filepath.Join(w.dir, f.path)
		if err := os.WriteFile(fullPath, f.new, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", f.path, err)
		}
	}

	return nil
}
