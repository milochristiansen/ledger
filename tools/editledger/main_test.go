package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/tools"
)

func makeEditableTx() *ledger.Transaction {
	d, _ := time.Parse("2006/01/02", "2025/08/20")
	cd, _ := time.Parse("2006/01/02", "2025/08/22")
	return &ledger.Transaction{
		Date:        d,
		ClearDate:   cd,
		Status:      ledger.StatusClear,
		Code:        "TX123",
		Description: "Amazon",
		Postings: []ledger.Posting{
			{Account: "Expenses:Other", Value: 47924, Null: false},
			{Account: "Assets:Checking", Value: -47924, Null: false, HasAssert: true, Assert: 100000},
		},
	}
}

func TestApplyEdit_Description(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "description", "New Payee"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Description != "New Payee" {
		t.Errorf("description = %q, want %q", tx.Description, "New Payee")
	}
	wantDate := "2025/08/20"
	if got := tx.Date.Format("2006/01/02"); got != wantDate {
		t.Errorf("date changed to %s, want %s", got, wantDate)
	}
}

func TestApplyEdit_Date(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "date", "2025/09/15"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tx.Date.Format("2006/01/02"); got != "2025/09/15" {
		t.Errorf("date = %s, want 2025/09/15", got)
	}
}

func TestApplyEdit_DateBad(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "date", "not-a-date"); err == nil {
		t.Error("expected error for bad date")
	}
}

func TestApplyEdit_ClearDate(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "clear_date", "2025/09/01"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tx.ClearDate.Format("2006/01/02"); got != "2025/09/01" {
		t.Errorf("clear_date = %s, want 2025/09/01", got)
	}
}

func TestApplyEdit_Status(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"clear", 2, false},
		{"*", 2, false},
		{"pending", 1, false},
		{"!", 1, false},
		{"none", 0, false},
		{"", 0, false},
		{"cleared", 0, true},
		{"bogus", 0, true},
	}
	for _, tt := range tests {
		tx := makeEditableTx()
		err := applyEdit(tx, "status", tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("status=%q expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("status=%q unexpected error: %v", tt.input, err)
			continue
		}
		if int(tx.Status) != tt.want {
			t.Errorf("status=%q got %d, want %d", tt.input, tx.Status, tt.want)
		}
	}
}

func TestApplyEdit_Code(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "code", "NEWCODE"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Code != "NEWCODE" {
		t.Errorf("code = %q, want %q", tx.Code, "NEWCODE")
	}
}

func TestApplyEdit_Account(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "account", "Expenses:Food"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].Account != "Expenses:Food" {
		t.Errorf("account = %q, want Expenses:Food", tx.Postings[0].Account)
	}
	if err := applyEdit(tx, "account:1", "Assets:Savings"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[1].Account != "Assets:Savings" {
		t.Errorf("account:1 = %q, want Assets:Savings", tx.Postings[1].Account)
	}
}

func TestApplyEdit_AccountOutOfRange(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "account:5", "Expenses:Foo"); err == nil {
		t.Error("expected error for out-of-range account index")
	}
}

func TestApplyEdit_Amount(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "amount", "12.50"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].Value != 125000 {
		t.Errorf("amount value = %d, want 125000", tx.Postings[0].Value)
	}
	if tx.Postings[0].Null {
		t.Error("amount should not be null")
	}
}

func TestApplyEdit_AmountEmpty(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "amount", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tx.Postings[0].Null {
		t.Error("amount should be null after empty value")
	}
}

func TestApplyEdit_AmountN(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "amount:1", "-50.00"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[1].Value != -500000 {
		t.Errorf("amount:1 value = %d, want -500000", tx.Postings[1].Value)
	}
}

func TestApplyEdit_Assert(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "assert", "200.00"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].Assert != 2000000 {
		t.Errorf("assert = %d, want 2000000", tx.Postings[0].Assert)
	}
	if !tx.Postings[0].HasAssert {
		t.Error("HasAssert should be true")
	}
}

func TestApplyEdit_AssertClear(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "assert:1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[1].HasAssert {
		t.Error("HasAssert should be false after clearing")
	}
}

func TestApplyEdit_Note(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "note", "foo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].Note != "foo" {
		t.Errorf("note = %q, want foo", tx.Postings[0].Note)
	}
}

func TestApplyEdit_NoteN(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "note:1", "bar"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[1].Note != "bar" {
		t.Errorf("note:1 = %q, want bar", tx.Postings[1].Note)
	}
}

func TestApplyEdit_AmountNEmpty(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "amount:1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tx.Postings[1].Null {
		t.Error("amount:1 should be null after empty value")
	}
	if tx.Postings[1].Value != 0 {
		t.Errorf("amount:1 value = %d, want 0", tx.Postings[1].Value)
	}
}

func TestApplyEdit_UnknownField(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "bogus", "value"); err == nil {
		t.Error("expected error for unknown field")
	}
}

func TestInsertOrDeletePosting_Insert(t *testing.T) {
	tx := makeEditableTx()
	if err := insertOrDeletePosting(tx, "1", "Expenses:Guns"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tx.Postings) != 3 {
		t.Fatalf("expected 3 postings, got %d", len(tx.Postings))
	}
	if tx.Postings[1].Account != "Expenses:Guns" {
		t.Errorf("posting[1] account = %q", tx.Postings[1].Account)
	}
	if !tx.Postings[1].Null {
		t.Error("new posting should be null")
	}
	if tx.Postings[2].Account != "Assets:Checking" {
		t.Errorf("posting[2] account = %q, want Assets:Checking", tx.Postings[2].Account)
	}
}

func TestInsertOrDeletePosting_Delete(t *testing.T) {
	tx := makeEditableTx()
	if err := insertOrDeletePosting(tx, "0", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tx.Postings) != 1 {
		t.Fatalf("expected 1 posting, got %d", len(tx.Postings))
	}
	if tx.Postings[0].Account != "Assets:Checking" {
		t.Errorf("remaining posting account = %q", tx.Postings[0].Account)
	}
}

func TestInsertOrDeletePosting_Append(t *testing.T) {
	tx := makeEditableTx()
	if err := insertOrDeletePosting(tx, "2", "Expenses:Food"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tx.Postings) != 3 {
		t.Fatalf("expected 3 postings, got %d", len(tx.Postings))
	}
	if tx.Postings[2].Account != "Expenses:Food" {
		t.Errorf("appended posting account = %q", tx.Postings[2].Account)
	}
}

func TestInsertOrDeletePosting_DeleteOutOfRange(t *testing.T) {
	tx := makeEditableTx()
	if err := insertOrDeletePosting(tx, "2", ""); err == nil {
		t.Error("expected error for delete at len (out of range)")
	}
}

func TestInsertOrDeletePosting_OutOfRange(t *testing.T) {
	tx := makeEditableTx()
	if err := insertOrDeletePosting(tx, "5", "Expenses:Foo"); err == nil {
		t.Error("expected error for out-of-range index")
	}
	if err := insertOrDeletePosting(tx, "-1", "Expenses:Foo"); err == nil {
		t.Error("expected error for negative index")
	}
}

func TestSetTxStatus(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"clear", 2, false},
		{"*", 2, false},
		{"pending", 1, false},
		{"!", 1, false},
		{"none", 0, false},
		{"", 0, false},
		{"cleared", 0, true},
		{"bogus", 0, true},
	}
	for _, tt := range tests {
		tx := &ledger.Transaction{Status: ledger.StatusClear}
		err := setTxStatus(tx, tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("setTxStatus(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("setTxStatus(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if int(tx.Status) != tt.want {
			t.Errorf("setTxStatus(%q) = %d, want %d", tt.input, tx.Status, tt.want)
		}
	}
}

func TestSetPostingAmount(t *testing.T) {
	tx := makeEditableTx()
	if err := setPostingAmount(tx, 0, "99.99"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].Value != 999900 {
		t.Errorf("value = %d, want 999900", tx.Postings[0].Value)
	}
	if tx.Postings[0].Null {
		t.Error("should not be null")
	}
}

func TestSetPostingAmount_Empty(t *testing.T) {
	tx := makeEditableTx()
	if err := setPostingAmount(tx, 0, ""); err == nil {
		t.Error("expected error for empty amount")
	}
}

func TestSetPostingAssert(t *testing.T) {
	tx := makeEditableTx()
	if err := setPostingAssert(tx, 0, "50.00"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].Assert != 500000 {
		t.Errorf("assert = %d, want 500000", tx.Postings[0].Assert)
	}
	if !tx.Postings[0].HasAssert {
		t.Error("HasAssert should be true")
	}
}

func TestSetPostingAssert_Clear(t *testing.T) {
	tx := makeEditableTx()
	if err := setPostingAssert(tx, 0, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].HasAssert {
		t.Error("HasAssert should be false after clearing")
	}
}

func TestApplyEdit_Posting(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, "posting:1", "Expenses:Food"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tx.Postings) != 3 {
		t.Fatalf("expected 3 postings, got %d", len(tx.Postings))
	}
	if tx.Postings[1].Account != "Expenses:Food" {
		t.Errorf("posting[1] account = %q, want Expenses:Food", tx.Postings[1].Account)
	}
}

func TestApplyEdit_AccountNoPostings(t *testing.T) {
	tx := &ledger.Transaction{
		Date:        makeEditableTx().Date,
		Description: "Test",
		Postings:    []ledger.Posting{},
	}
	if err := applyEdit(tx, "account", "Expenses:Food"); err == nil {
		t.Error("expected error for account on tx with no postings")
	} else if err.Error() != "no postings" {
		t.Errorf("error = %q, want %q", err.Error(), "no postings")
	}
}

func TestApplyEdit_AssertNoPostings(t *testing.T) {
	tx := &ledger.Transaction{
		Date:        makeEditableTx().Date,
		Description: "Test",
		Postings:    []ledger.Posting{},
	}
	if err := applyEdit(tx, "assert", "100.00"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertOrDeletePosting_NonNumericIndex(t *testing.T) {
	tx := makeEditableTx()
	if err := insertOrDeletePosting(tx, "abc", "Expenses:Food"); err == nil {
		t.Error("expected error for non-numeric index")
	}
}

func TestSetPostingAmount_BadFormat(t *testing.T) {
	tx := makeEditableTx()
	if err := setPostingAmount(tx, 0, "$."); err == nil {
		t.Error("expected error for bad amount format")
	}
}

func TestSetPostingAssert_BadFormat(t *testing.T) {
	tx := makeEditableTx()
	if err := setPostingAssert(tx, 0, "$."); err == nil {
		t.Error("expected error for bad assert format")
	}
}

func TestMakeRef_WithClearDate(t *testing.T) {
	tx := makeEditableTx()
	ref := makeRef("test.ledger", 0, tx)
	if ref == "" {
		t.Error("expected non-empty ref")
	}
}

func TestMakeRef_WithoutClearDate(t *testing.T) {
	tx := makeEditableTx()
	tx.ClearDate = time.Time{}
	ref := makeRef("test.ledger", 0, tx)
	if ref == "" {
		t.Error("expected non-empty ref")
	}
	// Verify the hash differs from the same tx with ClearDate set.
	txWithCD := makeEditableTx()
	refWithCD := makeRef("test.ledger", 0, txWithCD)
	if ref == refWithCD {
		t.Error("hash should differ when ClearDate is zero vs non-zero")
	}
}

// ---- filesystem tests ----

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

// firstRef parses the ledger file and returns the ref of the first transaction.
func firstRef(t *testing.T, rootPath string) string {
	t.Helper()
	w, err := tools.NewFileSafeWriter(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Includes(w.Add)
	if err != nil {
		t.Fatal(err)
	}
	for i, ent := range w.File.Entries {
		tx, ok := ent.(*ledger.Transaction)
		if !ok {
			continue
		}
		tc := tx.CleanCopy()
		return fmt.Sprintf("%d:%s", i, makeRef(filepath.Base(rootPath), i, tc))
	}
	t.Fatal("no transactions found")
	return ""
}

func resetEditFlags() {
	*refFlag = ""
	*fileFlag = ""
	sets = nil
}

func TestEdit_Description(t *testing.T) {
	defer resetEditFlags()

	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedger)

	ref := firstRef(t, path)
	*refFlag = ref
	sets = flagValue{{"description", "Amazon (Return)"}}

	newRef, err := edit(path)
	if err != nil {
		t.Fatal(err)
	}
	if newRef == "" {
		t.Error("edit returned empty ref")
	}
	if newRef == ref {
		t.Error("ref should change after edit")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == testLedger {
		t.Error("file content should have changed")
	}
}

func TestEdit_NotFound(t *testing.T) {
	defer resetEditFlags()

	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedger)

	*refFlag = "999:deadbeef"
	sets = flagValue{{"description", "nope"}}

	_, err := edit(path)
	if err == nil {
		t.Error("expected error for unknown ref")
	}
}

func TestEdit_MultipleSets(t *testing.T) {
	defer resetEditFlags()

	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedger)

	ref := firstRef(t, path)
	*refFlag = ref
	sets = flagValue{
		{"description", "Amazon Return"},
		{"account:0", "Expenses:Returns"},
	}

	newRef, err := edit(path)
	if err != nil {
		t.Fatal(err)
	}
	if newRef == "" {
		t.Error("edit returned empty ref")
	}
}

func TestEdit_FileScopeFound(t *testing.T) {
	defer resetEditFlags()

	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedger)

	ref := firstRef(t, path)
	*refFlag = ref
	*fileFlag = "test.ledger"
	sets = flagValue{{"description", "Amazon (Return)"}}

	newRef, err := edit(path)
	if err != nil {
		t.Fatal(err)
	}
	if newRef == "" {
		t.Error("edit returned empty ref")
	}
}

func TestEdit_FileScopeNotFound(t *testing.T) {
	defer resetEditFlags()

	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedger)

	ref := firstRef(t, path)
	*refFlag = ref
	*fileFlag = "wrong.ledger"
	sets = flagValue{{"description", "nope"}}

	_, err := edit(path)
	if err == nil {
		t.Error("expected error when -file scope does not match")
	}
}

func TestEdit_FileScopeWrongFile(t *testing.T) {
	defer resetEditFlags()

	dir := t.TempDir()
	rootPath := writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)

	// Get a ref from the ROOT file
	rootRef := firstRefMulti(t, rootPath)
	*refFlag = rootRef
	*fileFlag = "sub.ledger" // scope to sub file, but ref is from root
	sets = flagValue{{"description", "nope"}}

	_, err := edit(rootPath)
	if err == nil {
		t.Error("expected error when -file scope matches but ref not in that file")
	}
}

// firstRefMulti returns the ref of the first transaction in the root file (not includes).
func firstRefMulti(t *testing.T, rootPath string) string {
	t.Helper()
	w, err := tools.NewFileSafeWriter(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Includes(w.Add)
	if err != nil {
		t.Fatal(err)
	}
	for i, ent := range w.File.Entries {
		tx, ok := ent.(*ledger.Transaction)
		if !ok {
			continue
		}
		tc := tx.CleanCopy()
		return fmt.Sprintf("%d:%s", i, makeRef(filepath.Base(rootPath), i, tc))
	}
	t.Fatal("no transactions found")
	return ""
}
// ---- multi-file (include directive) tests ----

const rootWithInclude = "include sub.ledger\n\n2025/08/20 Amazon\n    Expenses:Electronics    $47.92\n    Assets:Checking        -$47.92\n"
const subLedger = "2025/08/22 Groceries\n    Expenses:Food           $12.50\n    Assets:Checking        -$12.50\n"

// subRef returns the ref of the first transaction in the included sub-file.
func subRef(t *testing.T, dir string) string {
	t.Helper()
	rootPath := writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)

	w, err := tools.NewFileSafeWriter(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	pis, err := w.Includes(w.Add)
	if err != nil {
		t.Fatal(err)
	}
	for _, pi := range pis {
		if pi.File == nil {
			continue
		}
		for i, ent := range pi.File.Entries {
			tx, ok := ent.(*ledger.Transaction)
			if !ok {
				continue
			}
			return fmt.Sprintf("%d:%s", i, makeRef(pi.Path, i, tx))
		}
	}
	t.Fatal("no transaction found in included files")
	return ""
}

func TestEdit_MultiFile(t *testing.T) {
	defer resetEditFlags()

	dir := t.TempDir()
	rootPath := writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)

	ref := subRef(t, dir)
	*refFlag = ref
	sets = flagValue{{"description", "Groceries (Modified)"}}

	newRef, err := edit(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if newRef == "" {
		t.Error("edit returned empty ref")
	}
	if newRef == ref {
		t.Error("ref should change after edit")
	}

	// Verify the sub file was modified
	data, err := os.ReadFile(filepath.Join(dir, "sub.ledger"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Groceries (Modified)") {
		t.Error("sub file should contain modified description")
	}
}
