package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/milochristiansen/ledger"
)

const testLedger = `2025/08/20 Amazon
    Expenses:Electronics    $47.92
    Assets:Checking        -$47.92

2025/08/22 Groceries
    Expenses:Food           $12.50
    Assets:Checking        -$12.50
`

func writeTempLedger(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFileSafeWriter_Commit(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedger)

	w, err := NewFileSafeWriter(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = w.Includes(w.Add)
	if err != nil {
		t.Fatal(err)
	}

	// Modify the first transaction's description
	for _, ent := range w.File.Entries {
		if tx, ok := ent.(*ledger.Transaction); ok {
			tx.Description = "Modified"
			break
		}
	}

	if err := w.Commit(); err != nil {
		t.Fatal(err)
	}

	// Verify file was rewritten
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Modified") {
		t.Error("file should contain modified description")
	}

	// Verify backup was created
	backups, err := filepath.Glob(filepath.Join(dir, "backup-*.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
}

func TestFileSafeWriter_NoChange(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedger)

	// First pass: commit to canonicalize the format
	w, err := NewFileSafeWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Includes(w.Add)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Commit(); err != nil {
		t.Fatal(err)
	}

	// Second pass: no edits → Commit should detect no change
	w2, err := NewFileSafeWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w2.Includes(w2.Add)
	if err != nil {
		t.Fatal(err)
	}
	if err := w2.Commit(); err != nil {
		t.Fatal(err)
	}

	// Only the first commit's backup should exist
	backups, err := filepath.Glob(filepath.Join(dir, "backup-*.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Errorf("expected 1 backup (from canonicalization only), got %d", len(backups))
	}
}

func TestFileSafeWriter_NewFileSafeWriter_Missing(t *testing.T) {
	_, err := NewFileSafeWriter("/nonexistent/path/file.ledger")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestFileSafeWriter_NewFileSafeWriter_Unparseable(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "bad.ledger", "2021/09/25")

	_, err := NewFileSafeWriter(path)
	if err == nil {
		t.Fatal("expected error for unparseable ledger")
	}
	// Verify the error wraps a parse failure, not a file-not-found
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("expected parse error, got: %v", err)
	}
}
