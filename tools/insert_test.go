package tools

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/milochristiansen/ledger"
)

// TestInsert_Append inserts without positioning refs; verifies the new
// transaction is appended to the end of the file.
func TestInsert_Append(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "New Restaurant",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$35.00"},
			{Account: "Assets:Checking"},
		},
	}

	newRef, err := Insert(path, "", "", "", spec)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if newRef == "" {
		t.Fatal("empty ref returned")
	}

	// Verify it's findable
	result, err := QueryByRef(path, newRef, "")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("transaction not found by ref")
	}
	if result.Entry.Description != "New Restaurant" {
		t.Errorf("description = %q, want New Restaurant", result.Entry.Description)
	}

	// Verify it's the last transaction in the file
	w, err := NewFileSafeWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Includes(w.Add)
	if err != nil {
		t.Fatal(err)
	}
	last := w.File.Entries[len(w.File.Entries)-1]
	lastTx, ok := last.(*ledger.Transaction)
	if !ok {
		t.Fatal("last entry is not a transaction")
	}
	if lastTx.Description != "New Restaurant" {
		t.Errorf("last entry description = %q, want New Restaurant", lastTx.Description)
	}
}

// TestInsert_AfterRef inserts after a known transaction and verifies ordering.
func TestInsert_AfterRef(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	ref := firstRef(t, path) // first tx is "Amazon"

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "After Amazon",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	newRef, err := Insert(path, "", "", ref, spec)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	result, err := QueryByRef(path, newRef, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Entry.Description != "After Amazon" {
		t.Errorf("description = %q", result.Entry.Description)
	}

	// Verify it appears right after the first transaction
	w, err := NewFileSafeWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Includes(w.Add)
	if err != nil {
		t.Fatal(err)
	}
	tx1, _ := w.File.Entries[0].(*ledger.Transaction)
	tx2, _ := w.File.Entries[1].(*ledger.Transaction)
	if tx1.Description != "Amazon" {
		t.Errorf("entries[0] = %q, want Amazon", tx1.Description)
	}
	if tx2.Description != "After Amazon" {
		t.Errorf("entries[1] = %q, want After Amazon", tx2.Description)
	}
}

// TestInsert_BeforeRef inserts before a known transaction and verifies ordering.
func TestInsert_BeforeRef(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	ref := firstRef(t, path) // first tx is "Amazon"

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Before Amazon",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	newRef, err := Insert(path, "", ref, "", spec)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	result, err := QueryByRef(path, newRef, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Entry.Description != "Before Amazon" {
		t.Errorf("description = %q", result.Entry.Description)
	}

	// Verify it appears before the first transaction
	w, err := NewFileSafeWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Includes(w.Add)
	if err != nil {
		t.Fatal(err)
	}
	tx0, _ := w.File.Entries[0].(*ledger.Transaction)
	tx1, _ := w.File.Entries[1].(*ledger.Transaction)
	if tx0.Description != "Before Amazon" {
		t.Errorf("entries[0] = %q, want Before Amazon", tx0.Description)
	}
	if tx1.Description != "Amazon" {
		t.Errorf("entries[1] = %q, want Amazon", tx1.Description)
	}
}

// TestInsert_BothRefsError verifies that providing both beforeRef and afterRef
// returns an error.
func TestInsert_BothRefsError(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	ref := firstRef(t, path)
	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Test",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	_, err := Insert(path, "", ref, ref, spec)
	if err == nil {
		t.Fatal("expected error for both before_ref and after_ref set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %v, want mutually exclusive", err)
	}
}

// TestInsert_TargetFileNotFound verifies error when targetFile doesn't exist.
func TestInsert_TargetFileNotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Test",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	_, err := Insert(path, "nonexistent.ledger", "", "", spec)
	if err == nil {
		t.Fatal("expected error for nonexistent target file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want not found", err)
	}
}

// TestInsert_RefNotFound verifies error when the positioning ref doesn't exist.
func TestInsert_RefNotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Test",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	_, err := Insert(path, "", "999:deadbeef", "", spec)
	if err == nil {
		t.Fatal("expected error for nonexistent before_ref")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want not found", err)
	}

	_, err = Insert(path, "", "", "999:deadbeef", spec)
	if err == nil {
		t.Fatal("expected error for nonexistent after_ref")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want not found", err)
	}
}

// TestInsert_NoDate verifies error when date is empty.
func TestInsert_NoDate(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "",
		Description: "Test",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	_, err := Insert(path, "", "", "", spec)
	if err == nil {
		t.Fatal("expected error for empty date")
	}
}

// TestInsert_NoDescription verifies error when description is empty.
func TestInsert_NoDescription(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	_, err := Insert(path, "", "", "", spec)
	if err == nil {
		t.Fatal("expected error for empty description")
	}
}

// TestInsert_NoPostings verifies error when postings slice is empty.
func TestInsert_NoPostings(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Test",
		Postings:    nil,
	}

	_, err := Insert(path, "", "", "", spec)
	if err == nil {
		t.Fatal("expected error for empty postings")
	}
}

// TestInsert_NullPosting verifies that omitting the amount on a posting
// creates a null posting (balanced by implicit value).
func TestInsert_NullPosting(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Null Posting Test",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$50.00"},
			{Account: "Assets:Checking"}, // null
		},
	}

	newRef, err := Insert(path, "", "", "", spec)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	result, err := QueryByRef(path, newRef, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entry.Postings) != 2 {
		t.Fatalf("expected 2 postings, got %d", len(result.Entry.Postings))
	}
	if !result.Entry.Postings[1].Null {
		t.Error("second posting should be null")
	}
}

// TestInsert_FullFeatured tests a transaction with all optional fields:
// clear_date, status, code, comment, tags, kv, and posting-level status/note/assert.
func TestInsert_FullFeatured(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Full Featured",
		ClearDate:   "2025/09/20",
		Status:      "*",
		Code:        "TX123",
		Comment:     "This is a comment",
		Tags:        []string{"vacation", "reimbursable"},
		KV:          map[string]string{"Receipt": "98765", "Project": "Alpha"},
		Postings: []InsertPosting{
			{
				Account: "Expenses:Travel",
				Amount:  "$150.00",
				Note:    "Flight",
				Assert:  "$150.00",
				Status:  "*",
			},
			{
				Account: "Assets:Checking",
				Status:  "!",
			},
		},
	}

	newRef, err := Insert(path, "", "", "", spec)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	result, err := QueryByRef(path, newRef, "")
	if err != nil {
		t.Fatal(err)
	}

	tx := result.Entry
	if tx.Description != "Full Featured" {
		t.Errorf("description = %q", tx.Description)
	}
	if tx.Date.Format("2006/01/02") != "2025/09/15" {
		t.Errorf("date = %s", tx.Date.Format("2006/01/02"))
	}
	if tx.ClearDate.Format("2006/01/02") != "2025/09/20" {
		t.Errorf("clear_date = %s", tx.ClearDate.Format("2006/01/02"))
	}
	if tx.Status != ledger.StatusClear {
		t.Errorf("status = %d, want StatusClear", tx.Status)
	}
	if tx.Code != "TX123" {
		t.Errorf("code = %q", tx.Code)
	}
	if len(tx.Comments) != 1 || tx.Comments[0] != "This is a comment" {
		t.Errorf("comments = %v", tx.Comments)
	}
	if !tx.Tags["vacation"] || !tx.Tags["reimbursable"] {
		t.Errorf("tags = %v", tx.Tags)
	}
	if tx.KVPairs["Receipt"] != "98765" {
		t.Errorf("kv[Receipt] = %q", tx.KVPairs["Receipt"])
	}
	if tx.KVPairs["Project"] != "Alpha" {
		t.Errorf("kv[Project] = %q", tx.KVPairs["Project"])
	}

	if len(tx.Postings) != 2 {
		t.Fatalf("expected 2 postings, got %d", len(tx.Postings))
	}
	p0 := tx.Postings[0]
	if p0.Account != "Expenses:Travel" {
		t.Errorf("p0.account = %q", p0.Account)
	}
	if p0.Note != "Flight" {
		t.Errorf("p0.note = %q", p0.Note)
	}
	if p0.Status != ledger.StatusClear {
		t.Errorf("p0.status = %d", p0.Status)
	}
	if !p0.HasAssert {
		t.Error("p0 should have assert")
	}
	p1 := tx.Postings[1]
	if p1.Status != ledger.StatusPending {
		t.Errorf("p1.status = %d, want StatusPending", p1.Status)
	}
	if !p1.Null {
		t.Error("p1 should be null")
	}
}

// TestInsert_InvalidDate verifies that a malformed date returns an error.
func TestInsert_InvalidDate(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "not-a-date",
		Description: "Test",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	_, err := Insert(path, "", "", "", spec)
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

// TestInsert_InvalidStatus verifies that a bad status returns an error.
func TestInsert_InvalidStatus(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Test",
		Status:      "bogus",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	_, err := Insert(path, "", "", "", spec)
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

// TestInsert_InvalidPostingStatus verifies that a bad posting status returns an error.
func TestInsert_InvalidPostingStatus(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Test",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00", Status: "bogus"},
			{Account: "Assets:Checking"},
		},
	}

	_, err := Insert(path, "", "", "", spec)
	if err == nil {
		t.Fatal("expected error for invalid posting status")
	}
}

// TestInsert_BackupCreated verifies that a backup tar.gz is created on insert.
func TestInsert_BackupCreated(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Backup Test",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: "Assets:Checking"},
		},
	}

	_, err := Insert(path, "", "", "", spec)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	backups, err := filepath.Glob(filepath.Join(dir, "backup-*.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Errorf("expected 1 backup, got %d", len(backups))
	}
}

// TestInsert_MissingAccount verifies error when a posting lacks an account.
func TestInsert_MissingAccount(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	spec := InsertSpec{
		Date:        "2025/09/15",
		Description: "Test",
		Postings: []InsertPosting{
			{Account: "Expenses:Food", Amount: "$10.00"},
			{Account: ""}, // missing account
		},
	}

	_, err := Insert(path, "", "", "", spec)
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}
