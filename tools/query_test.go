package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse/lex"
)

// --- helper factories ---

func makeTestTx() *ledger.Transaction {
	d, _ := time.Parse("2006/01/02", "2025/08/20")
	cd, _ := time.Parse("2006/01/02", "2025/08/22")
	return &ledger.Transaction{
		Date:        d,
		ClearDate:   cd,
		Status:      ledger.StatusClear,
		Code:        "TX123",
		Description: "Amazon",
		Postings: []ledger.Posting{
			{Account: "Expenses:Electronics", Value: 479200, Null: false, Status: ledger.StatusClear},
			{Account: "Assets:Checking", Value: -479200, Null: false, HasAssert: true, Assert: 100000, Note: "Credit card charge"},
		},
		Location: lex.Location(0).L(42),
	}
}

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
			{Account: "Expenses:Other", Value: 479200, Null: false},
			{Account: "Assets:Checking", Value: -479200, Null: false, HasAssert: true, Assert: 100000},
		},
	}
}

func ptr[T any](v T) *T { return &v }

// --- parse helpers ---

func TestParseDateFlag(t *testing.T) {
	tests := []struct {
		input      string
		wantAfter  string
		wantBefore string
		wantErr    bool
	}{
		{"2025/08/24", "2025-08-24", "2025-08-25", false},
		{"2025/08/01:2025/08/31", "2025-08-01", "2025-09-01", false},
		{"2025/12/31:2025/01/01", "2025-01-01", "2026-01-01", false},
		{"bad", "", "", true},
		{"2025/08/01:bad", "", "", true},
		{"2025/08/01:", "", "", true},
	}
	for _, tt := range tests {
		after, before, err := parseDateFlag(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseDateFlag(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDateFlag(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got := after.Format("2006-01-02"); got != tt.wantAfter {
			t.Errorf("parseDateFlag(%q) after=%s, want %s", tt.input, got, tt.wantAfter)
		}
		if got := before.Format("2006-01-02"); got != tt.wantBefore {
			t.Errorf("parseDateFlag(%q) before=%s, want %s", tt.input, got, tt.wantBefore)
		}
	}
}

func TestParseAmountFlag(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"20.00", 200000, false},
		{"$20.00", 200000, false},
		{"-20.00", -200000, false},
		{"-$20.00", -200000, false},
		{"0", 0, false},
		{"", 0, true},
		{"$.", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		v, err := parseAmountFlag(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseAmountFlag(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseAmountFlag(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if v != tt.want {
			t.Errorf("parseAmountFlag(%q) = %d, want %d", tt.input, v, tt.want)
		}
	}
}

func TestParseStatusFlag(t *testing.T) {
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
		{"bogus", 0, true},
	}
	for _, tt := range tests {
		got, err := parseStatusFlag(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseStatusFlag(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseStatusFlag(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseStatusFlag(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// --- makeRef ---

func TestMakeRef(t *testing.T) {
	tx := makeTestTx()
	ref := makeRef("test.ledger", 0, tx)
	if ref == "" {
		t.Error("expected non-empty ref")
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
}

// --- CSVField ---

func TestCSVField(t *testing.T) {
	tx := makeTestTx()
	r := &QueryResult{
		File:  "test.ledger",
		Ref:   "0:abc123",
		Entry: tx,
	}
	tests := []struct {
		name string
		want string
	}{
		{"file", "test.ledger"},
		{"ref", "0:abc123"},
		{"date", "2025/08/20"},
		{"clear_date", "2025/08/22"},
		{"status", "*"},
		{"code", "TX123"},
		{"description", "Amazon"},
		{"account", "Expenses:Electronics"},
		{"account:0", "Expenses:Electronics"},
		{"account:1", "Assets:Checking"},
		{"note:1", "Credit card charge"},
		{"status:0", "*"},
	}
	for _, tt := range tests {
		got := CSVField(tt.name, r)
		if got != tt.want {
			t.Errorf("CSVField(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestCSVField_EdgeCases(t *testing.T) {
	t.Run("clear_date_zero", func(t *testing.T) {
		d, _ := time.Parse("2006/01/02", "2025/08/20")
		tx := &ledger.Transaction{
			Date:        d,
			Description: "Test",
			Postings: []ledger.Posting{
				{Account: "Expenses:Test", Value: 1000},
			},
			Location: lex.Location(0).L(10),
		}
		r := &QueryResult{Entry: tx}
		got := CSVField("clear_date", r)
		if got != "" {
			t.Errorf("expected empty string for zero ClearDate, got %q", got)
		}
	})

	t.Run("amount_null", func(t *testing.T) {
		d, _ := time.Parse("2006/01/02", "2025/08/20")
		tx := &ledger.Transaction{
			Date:        d,
			Description: "Test",
			Postings: []ledger.Posting{
				{Account: "Expenses:Test", Value: 0, Null: true},
			},
			Location: lex.Location(0).L(10),
		}
		r := &QueryResult{Entry: tx}
		got := CSVField("amount", r)
		if got != "" {
			t.Errorf("expected empty string for null posting, got %q", got)
		}
	})

	t.Run("account_oob", func(t *testing.T) {
		tx := makeTestTx()
		r := &QueryResult{Entry: tx}
		got := CSVField("account:5", r)
		if got != "" {
			t.Errorf("expected empty string for out-of-bounds account index, got %q", got)
		}
	})

	t.Run("assert_no_assert", func(t *testing.T) {
		tx := makeTestTx()
		r := &QueryResult{Entry: tx}
		got := CSVField("assert:0", r)
		if got != "" {
			t.Errorf("expected empty string for posting without assert, got %q", got)
		}
	})

	t.Run("comment", func(t *testing.T) {
		tx := makeTestTx()
		tx.Comments = []string{"hello"}
		r := &QueryResult{Entry: tx}
		got := CSVField("comment", r)
		if got != "hello" {
			t.Errorf("expected hello, got %q", got)
		}
	})

	t.Run("comment_empty", func(t *testing.T) {
		tx := makeTestTx()
		r := &QueryResult{Entry: tx}
		got := CSVField("comment", r)
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("tag_true", func(t *testing.T) {
		tx := makeTestTx()
		tx.Tags = map[string]bool{"vacation": true}
		r := &QueryResult{Entry: tx}
		got := CSVField("tag:vacation", r)
		if got != "true" {
			t.Errorf("expected true, got %q", got)
		}
	})

	t.Run("tag_false", func(t *testing.T) {
		tx := makeTestTx()
		r := &QueryResult{Entry: tx}
		got := CSVField("tag:vacation", r)
		if got != "false" {
			t.Errorf("expected false, got %q", got)
		}
	})

	t.Run("kv", func(t *testing.T) {
		tx := makeTestTx()
		tx.KVPairs = map[string]string{"Receipt": "12345"}
		r := &QueryResult{Entry: tx}
		got := CSVField("kv:Receipt", r)
		if got != "12345" {
			t.Errorf("expected 12345, got %q", got)
		}
	})

	t.Run("kv_missing", func(t *testing.T) {
		tx := makeTestTx()
		r := &QueryResult{Entry: tx}
		got := CSVField("kv:Receipt", r)
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

// --- applyEdit and sub-helpers (moved from editledger test) ---

func TestApplyEdit_Description(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{Description: ptr("New Payee")}); err != nil {
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
	if err := applyEdit(tx, EditSpec{Date: ptr("2025/09/15")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tx.Date.Format("2006/01/02"); got != "2025/09/15" {
		t.Errorf("date = %s, want 2025/09/15", got)
	}
}

func TestApplyEdit_DateBad(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{Date: ptr("not-a-date")}); err == nil {
		t.Error("expected error for bad date")
	}
}

func TestApplyEdit_ClearDate(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{ClearDate: ptr("2025/09/01")}); err != nil {
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
		err := applyEdit(tx, EditSpec{Status: ptr(tt.input)})
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
	if err := applyEdit(tx, EditSpec{Code: ptr("NEWCODE")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Code != "NEWCODE" {
		t.Errorf("code = %q, want %q", tx.Code, "NEWCODE")
	}
}

func TestApplyEdit_Account(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Account: ptr("Expenses:Food")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].Account != "Expenses:Food" {
		t.Errorf("account = %q, want Expenses:Food", tx.Postings[0].Account)
	}
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 1, Account: ptr("Assets:Savings")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[1].Account != "Assets:Savings" {
		t.Errorf("account:1 = %q, want Assets:Savings", tx.Postings[1].Account)
	}
}

func TestApplyEdit_AccountOutOfRange(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 5, Account: ptr("Expenses:Foo")}}}); err == nil {
		t.Error("expected error for out-of-range account index")
	}
}

func TestApplyEdit_Amount(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Amount: ptr("12.50")}}}); err != nil {
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
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Amount: ptr("")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tx.Postings[0].Null {
		t.Error("amount should be null after empty value")
	}
}

func TestApplyEdit_AmountN(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 1, Amount: ptr("-50.00")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[1].Value != -500000 {
		t.Errorf("amount:1 value = %d, want -500000", tx.Postings[1].Value)
	}
}

func TestApplyEdit_Assert(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Assert: ptr("200.00")}}}); err != nil {
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
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 1, Assert: ptr("")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[1].HasAssert {
		t.Error("HasAssert should be false after clearing")
	}
}

func TestApplyEdit_Note(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Note: ptr("foo")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].Note != "foo" {
		t.Errorf("note = %q, want foo", tx.Postings[0].Note)
	}
}

func TestApplyEdit_NoteN(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 1, Note: ptr("bar")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[1].Note != "bar" {
		t.Errorf("note:1 = %q, want bar", tx.Postings[1].Note)
	}
}

func TestApplyEdit_AmountNEmpty(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 1, Amount: ptr("")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tx.Postings[1].Null {
		t.Error("amount:1 should be null after empty value")
	}
	if tx.Postings[1].Value != 0 {
		t.Errorf("amount:1 value = %d, want 0", tx.Postings[1].Value)
	}
}

func TestInsertOrDeletePosting_Insert(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "insert", Index: 1, Account: ptr("Expenses:Guns")}}}); err != nil {
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
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "delete", Index: 0}}}); err != nil {
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
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "insert", Index: 2, Account: ptr("Expenses:Food")}}}); err != nil {
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
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "delete", Index: 2}}}); err == nil {
		t.Error("expected error for delete at len (out of range)")
	}
}

func TestInsertOrDeletePosting_OutOfRange(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "insert", Index: 5, Account: ptr("Expenses:Foo")}}}); err == nil {
		t.Error("expected error for out-of-range index")
	}
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "insert", Index: -1, Account: ptr("Expenses:Foo")}}}); err == nil {
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
		err := applyEdit(tx, EditSpec{Status: ptr(tt.input)})
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
			t.Errorf("status=%q = %d, want %d", tt.input, tx.Status, tt.want)
		}
	}
}

func TestSetPostingAmount(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Amount: ptr("99.99")}}}); err != nil {
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
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Amount: ptr("")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tx.Postings[0].Null {
		t.Error("amount should be null after empty value")
	}
}

func TestSetPostingAssert(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Assert: ptr("50.00")}}}); err != nil {
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
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Assert: ptr("")}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Postings[0].HasAssert {
		t.Error("HasAssert should be false after clearing")
	}
}

func TestApplyEdit_Posting(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "insert", Index: 1, Account: ptr("Expenses:Food")}}}); err != nil {
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
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Account: ptr("Expenses:Food")}}}); err == nil {
		t.Error("expected error for account on tx with no postings")
	} else if err.Error() != "posting index out of range: 0" {
		t.Errorf("error = %q, want %q", err.Error(), "posting index out of range: 0")
	}
}

func TestApplyEdit_AssertNoPostings(t *testing.T) {
	tx := &ledger.Transaction{
		Date:        makeEditableTx().Date,
		Description: "Test",
		Postings:    []ledger.Posting{},
	}
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Assert: ptr("100.00")}}}); err == nil {
		t.Error("expected error for assert on tx with no postings")
	} else if err.Error() != "posting index out of range: 0" {
		t.Errorf("error = %q, want %q", err.Error(), "posting index out of range: 0")
	}
}

func TestSetPostingAmount_BadFormat(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Amount: ptr("$.")}}}); err == nil {
		t.Error("expected error for bad amount format")
	}
}

func TestSetPostingAssert_BadFormat(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{PostingOps: []PostingOp{{Op: "set", Index: 0, Assert: ptr("$.")}}}); err == nil {
		t.Error("expected error for bad assert format")
	}
}

// --- filesystem helpers ---

// testLedgerMulti has 3 transactions (compared to fileio_test.go's testLedger with 2).
const testLedgerMulti = `2025/08/20 Amazon
    Expenses:Electronics    $47.92
    Assets:Checking        -$47.92

2025/08/22 * Groceries
    Expenses:Food           $12.50
    Assets:Checking        -$12.50

2025/09/01 ! Rent
    Expenses:Rent           $800.00
    Assets:Savings         -$800.00
`

// firstRef returns the ref of the first transaction found in the ledger tree.
func firstRef(t *testing.T, rootPath string) string {
	t.Helper()
	w, err := NewFileSafeWriter(rootPath)
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

// --- Query integration tests ---

func TestQuery_DateFilter(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Date: "2025/08/01:2025/08/31"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (Aug), got %d", len(results))
	}
}

func TestQuery_AccountFilter(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Account: "Food"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Description != "Groceries" {
		t.Errorf("expected Groceries, got %s", results[0].Entry.Description)
	}
}

func TestQuery_PayeeFilter(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Payee: "Amazon"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Description != "Amazon" {
		t.Errorf("expected Amazon, got %s", results[0].Entry.Description)
	}
}

func TestQuery_AmountExact(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Amount: "800.00"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Description != "Rent" {
		t.Errorf("expected Rent, got %s", results[0].Entry.Description)
	}
}

func TestQuery_AmountRange(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Amount: "10.00:50.00"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestQuery_AmountRangeSwap(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Amount: "50.00:10.00"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestQuery_ExcludeAccount(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{ExcludeAccount: "Checking"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Description != "Rent" {
		t.Errorf("expected Rent, got %s", results[0].Entry.Description)
	}
}

func TestQuery_ExcludePayee(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{ExcludePayee: "Amazon"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (Groceries + Rent), got %d", len(results))
	}
}

func TestQuery_AllFilters(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{
		Date:    "2025/08/01:2025/08/31",
		Account: "Food",
		Amount:  "10.00:20.00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Description != "Groceries" {
		t.Errorf("expected Groceries, got %s", results[0].Entry.Description)
	}
}

func TestQuery_NoMatches(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Account: "Nonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestQuery_StatusClear(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Status: "clear"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 clear transaction, got %d", len(results))
	}
	if results[0].Entry.Description != "Groceries" {
		t.Errorf("expected Groceries, got %s", results[0].Entry.Description)
	}
}

func TestQuery_StatusPending(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Status: "!"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 pending transaction, got %d", len(results))
	}
	if results[0].Entry.Description != "Rent" {
		t.Errorf("expected Rent, got %s", results[0].Entry.Description)
	}
}

func TestQuery_StatusNone(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Status: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 undefined-status transaction, got %d", len(results))
	}
	if results[0].Entry.Description != "Amazon" {
		t.Errorf("expected Amazon, got %s", results[0].Entry.Description)
	}
}

func TestQuery_ExcludeStatus(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{ExcludeStatus: "clear"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (excluding clear), got %d", len(results))
	}
}

func TestQuery_StatusInvalid(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	_, err := Query(path, QueryParams{Status: "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
}

// --- QueryByRef integration tests ---

func TestQueryByRef_Found(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	// Get a known ref by querying first
	results, err := Query(path, QueryParams{Date: "2025/08/01:2025/08/31"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results from query")
	}

	result, err := QueryByRef(path, results[0].Ref, "")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("QueryByRef returned nil")
	}
	if result.Entry.Description != results[0].Entry.Description {
		t.Errorf("wrong transaction: %s vs %s", result.Entry.Description, results[0].Entry.Description)
	}
}

func TestQueryByRef_NotFound(t *testing.T) {
	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	result, err := QueryByRef(filepath.Join(dir, "test.ledger"), "999:deadbeef", "")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil for unknown ref")
	}
}

func TestQueryByRef_WithScopeFile(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	results, err := Query(path, QueryParams{Date: "2025/08/01:2025/08/31"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results from query")
	}

	// Look up in wrong file -> not found
	result, err := QueryByRef(path, results[0].Ref, "nonexistent.ledger")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil when scoped to wrong file")
	}

	// Look up in correct file -> found
	result, err = QueryByRef(path, results[0].Ref, "test.ledger")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Error("expected result when scoped to correct file")
	}

	// Look up in correct file but with ref not in that file -> not found
	result, err = QueryByRef(path, "999:deadbeef", "test.ledger")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil when ref not found in scoped file")
	}
}

// --- Edit integration tests ---

func TestEdit_Description(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	ref := firstRef(t, path)
	newRef, err := Edit(path, ref, "", EditSpec{Description: ptr("Amazon (Return)")})
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
	if string(data) == testLedgerMulti {
		t.Error("file content should have changed")
	}

	// Verify backup was created
	backups, _ := filepath.Glob(filepath.Join(dir, "backup-*.tar.gz"))
	if len(backups) != 1 {
		t.Errorf("expected 1 backup, got %d", len(backups))
	}
}

func TestEdit_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	_, err := Edit(path, "999:deadbeef", "", EditSpec{Description: ptr("nope")})
	if err == nil {
		t.Error("expected error for unknown ref")
	}
}

func TestEdit_MultipleSets(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	ref := firstRef(t, path)
	newRef, err := Edit(path, ref, "", EditSpec{
		Description: ptr("Amazon Return"),
		PostingOps:  []PostingOp{{Op: "set", Index: 0, Account: ptr("Expenses:Returns")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if newRef == "" {
		t.Error("edit returned empty ref")
	}
}

func TestEdit_FileScopeFound(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	ref := firstRef(t, path)
	newRef, err := Edit(path, ref, "test.ledger", EditSpec{Description: ptr("Amazon (Return)")})
	if err != nil {
		t.Fatal(err)
	}
	if newRef == "" {
		t.Error("edit returned empty ref")
	}
}

func TestEdit_FileScopeNotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedgerMulti)

	ref := firstRef(t, path)
	_, err := Edit(path, ref, "wrong.ledger", EditSpec{Description: ptr("nope")})
	if err == nil {
		t.Error("expected error when -file scope does not match")
	}
}

func TestFormat(t *testing.T) {
	dir := t.TempDir()
	// Unformatted content (extra spaces + extra blank lines)
	content := "2025/08/20  Amazon\n    Expenses:Electronics    $47.92\n    Assets:Checking        -$47.92\n\n\n2025/08/22 * Groceries\n    Expenses:Food           $12.50\n    Assets:Checking        -$12.50\n\n"
	path := writeTempLedger(t, dir, "test.ledger", content)

	_, err := Format(path)
	if err != nil {
		t.Fatal(err)
	}

	// Verify backup created
	backups, _ := filepath.Glob(filepath.Join(dir, "backup-*.tar.gz"))
	if len(backups) != 1 {
		t.Errorf("expected 1 backup, got %d", len(backups))
	}

	// Read result
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	formatted := string(data)

	// Formatted: three spaces between date and description for unmarked transactions
	// (date, then "   " for status placeholder).
	if !strings.Contains(formatted, "2025/08/20   Amazon") {
		t.Errorf("expected formatted output with triple-space, got: %s", formatted)
	}
	if strings.Contains(formatted, "\n\n\n") {
		t.Error("extra blank lines not removed")
	}
}

// --- multi-file (include directive) tests ---

const rootWithInclude = `include sub.ledger

2025/08/20 Amazon
    Expenses:Electronics    $47.92
    Assets:Checking        -$47.92
`

const subLedger = `2025/08/22 Groceries
    Expenses:Food           $12.50
    Assets:Checking        -$12.50
`

func TestQuery_WithInclude(t *testing.T) {
	dir := t.TempDir()
	writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)

	results, err := Query(filepath.Join(dir, "root.ledger"), QueryParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 transactions (root + sub), got %d", len(results))
	}
}

func TestQueryByRef_MultiFile(t *testing.T) {
	dir := t.TempDir()
	rootPath := writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)

	results, err := Query(rootPath, QueryParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatal("not enough results")
	}

	// Find the sub-ledger transaction by ref (unscoped)
	subResult := results[1] // Groceries from sub.ledger
	result, err := QueryByRef(rootPath, subResult.Ref, "")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected to find transaction in included file")
	}
	if result.Entry.Description != "Groceries" {
		t.Errorf("expected Groceries, got %s", result.Entry.Description)
	}
}

func TestEdit_MultiFile(t *testing.T) {
	dir := t.TempDir()
	rootPath := writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)

	// Get ref from the sub file
	ref := subRef(t, dir)

	newRef, err := Edit(rootPath, ref, "", EditSpec{Description: ptr("Groceries (Modified)")})
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

func TestEdit_FileScopeWrongFile(t *testing.T) {
	dir := t.TempDir()
	rootPath := writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)

	// Get a ref from the ROOT file
	rootRef := firstRefMulti(t, rootPath)
	_, err := Edit(rootPath, rootRef, "sub.ledger", EditSpec{Description: ptr("nope")})
	if err == nil {
		t.Error("expected error when -file scope matches but ref not in that file")
	}
}

// subRef returns the ref of the first transaction in the included sub-file.
func subRef(t *testing.T, dir string) string {
	t.Helper()
	rootPath := writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)

	w, err := NewFileSafeWriter(rootPath)
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

// firstRefMulti returns the ref of the first transaction in the root file (not includes).
func firstRefMulti(t *testing.T, rootPath string) string {
	t.Helper()
	w, err := NewFileSafeWriter(rootPath)
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

// --- tag/kv/comment tests ---

const taggedLedger = `2025/08/20 Amazon
	; :vacation:
	; Receipt: 12345
	Expenses:Electronics    $47.92
	Assets:Checking        -$47.92

2025/08/22 Groceries
	; :food:
	; Store: Kroger
	Expenses:Food           $12.50
	Assets:Checking        -$12.50
`

func TestQuery_Tag(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", taggedLedger)

	results, err := Query(path, QueryParams{Tag: "vacation"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Description != "Amazon" {
		t.Errorf("expected Amazon, got %s", results[0].Entry.Description)
	}

	// tag not present
	results, err = Query(path, QueryParams{Tag: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestQuery_KV(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", taggedLedger)

	// key only
	results, err := Query(path, QueryParams{KV: "Receipt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Description != "Amazon" {
		t.Errorf("expected Amazon, got %s", results[0].Entry.Description)
	}

	// key + value
	results, err = Query(path, QueryParams{KV: "Store:Kroger"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Description != "Groceries" {
		t.Errorf("expected Groceries, got %s", results[0].Entry.Description)
	}

	// key exists but wrong value
	results, err = Query(path, QueryParams{KV: "Store:Walmart"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestApplyEdit_Comment(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{Comment: ptr("New comment")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tx.Comments) != 1 || tx.Comments[0] != "New comment" {
		t.Errorf("Comments = %v, want [New comment]", tx.Comments)
	}

	// clear
	if err := applyEdit(tx, EditSpec{Comment: ptr("")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Comments != nil {
		t.Errorf("Comments = %v, want nil", tx.Comments)
	}
}

func TestApplyEdit_Tag(t *testing.T) {
	tx := makeEditableTx()
	if err := applyEdit(tx, EditSpec{TagOps: []TagOp{{Op: "add", Name: "vacation"}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tx.Tags["vacation"] {
		t.Error("tag vacation not set")
	}
	if err := applyEdit(tx, EditSpec{TagOps: []TagOp{{Op: "add", Name: "food"}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tx.Tags["food"] {
		t.Error("tag food not set")
	}
	if len(tx.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tx.Tags))
	}
	// remove
	if err := applyEdit(tx, EditSpec{TagOps: []TagOp{{Op: "remove", Name: "food"}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Tags["food"] {
		t.Error("tag food should be removed")
	}
	if len(tx.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tx.Tags))
	}
}
func TestApplyEdit_KV(t *testing.T) {
	tx := makeEditableTx()
	// set
	if err := applyEdit(tx, EditSpec{KVOps: []KVOp{{Op: "set", Key: "Receipt", Value: "12345"}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.KVPairs["Receipt"] != "12345" {
		t.Errorf("KVPairs[Receipt] = %q, want 12345", tx.KVPairs["Receipt"])
	}

	// overwrite
	if err := applyEdit(tx, EditSpec{KVOps: []KVOp{{Op: "set", Key: "Receipt", Value: "67890"}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.KVPairs["Receipt"] != "67890" {
		t.Errorf("KVPairs[Receipt] = %q, want 67890", tx.KVPairs["Receipt"])
	}

	// delete
	if err := applyEdit(tx, EditSpec{KVOps: []KVOp{{Op: "delete", Key: "Receipt"}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tx.KVPairs["Receipt"]; ok {
		t.Error("Receipt key should be deleted")
	}
}

// --- matchFilter unit tests ---

func TestMatchFilter_Leaf_AccountRegex(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "account", Match: "regex", Arg1: "Electronics"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for account regex Electronics")
	}
}

func TestMatchFilter_Leaf_AccountExact(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "account", Match: "exact", Arg1: "Expenses:Electronics"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for exact account Expenses:Electronics")
	}
}

func TestMatchFilter_Leaf_AccountExactNoMatch(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "account", Match: "exact", Arg1: "Expenses:Food"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no match for account Expenses:Food")
	}
}

func TestMatchFilter_Leaf_AccountInvert(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "account", Match: "regex", Arg1: "Food", Invert: true}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match because invert negates no-match on Food")
	}
}

func TestMatchFilter_Leaf_PayeeRegex(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "payee", Match: "regex", Arg1: "Amaz"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for payee regex Amaz")
	}
}

func TestMatchFilter_Leaf_PayeeExact(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "payee", Match: "exact", Arg1: "Amazon"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for exact payee Amazon")
	}
}

func TestMatchFilter_Leaf_DateExact(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "date", Match: "exact", Arg1: "2025/08/20"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for exact date 2025/08/20")
	}
}

func TestMatchFilter_Leaf_DateRange(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "date", Match: "range", Arg1: "2025/08/19", Arg2: "2025/08/21"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for date range 2025/08/19-2025/08/21")
	}
}

func TestMatchFilter_Leaf_DateRangeOutOfBounds(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "date", Match: "range", Arg1: "2025/08/21", Arg2: "2025/08/22"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no match for date range after transaction")
	}
}

func TestMatchFilter_UnknownField(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "bogus", Match: "exact", Arg1: "x"}
	_, err := matchFilter(tx, fn)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestMatchFilter_InvalidRegex(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "account", Match: "regex", Arg1: "[invalid"}
	_, err := matchFilter(tx, fn)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestMatchFilter_Leaf_AmountExact(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "amount", Match: "exact", Arg1: "47.92"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for exact amount 47.92")
	}
}

func TestMatchFilter_Leaf_AmountRange(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "amount", Match: "range", Arg1: "40.00", Arg2: "50.00"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for amount range 40.00-50.00")
	}
}

func TestMatchFilter_Leaf_Status(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "status", Match: "exact", Arg1: "clear"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for status clear")
	}
}

func TestMatchFilter_Leaf_StatusNoMatch(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{Field: "status", Match: "exact", Arg1: "pending"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no match for status pending (tx is clear)")
	}
}

func TestMatchFilter_Leaf_Tag(t *testing.T) {
	tx := makeTestTx()
	tx.Tags = map[string]bool{"vacation": true}
	fn := FilterNode{Field: "tag", Match: "has", Arg1: "vacation"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for tag vacation")
	}
}

func TestMatchFilter_Leaf_TagMissing(t *testing.T) {
	tx := makeTestTx()
	tx.Tags = map[string]bool{}
	fn := FilterNode{Field: "tag", Match: "has", Arg1: "vacation"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no match for missing tag")
	}
}

func TestMatchFilter_Leaf_KV_Has(t *testing.T) {
	tx := makeTestTx()
	tx.KVPairs = map[string]string{"Receipt": "12345"}
	fn := FilterNode{Field: "kv", Match: "has", Arg1: "Receipt"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for kv has Receipt")
	}
}

func TestMatchFilter_Leaf_KV_HasMissing(t *testing.T) {
	tx := makeTestTx()
	tx.KVPairs = map[string]string{}
	fn := FilterNode{Field: "kv", Match: "has", Arg1: "Receipt"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no match for missing kv key")
	}
}

func TestMatchFilter_Leaf_KV_Exact(t *testing.T) {
	tx := makeTestTx()
	tx.KVPairs = map[string]string{"Receipt": "12345"}
	fn := FilterNode{Field: "kv", Match: "exact", Arg1: "Receipt", Arg2: "12345"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for kv exact Receipt=12345")
	}
}

func TestMatchFilter_Leaf_KV_ExactWrongValue(t *testing.T) {
	tx := makeTestTx()
	tx.KVPairs = map[string]string{"Receipt": "12345"}
	fn := FilterNode{Field: "kv", Match: "exact", Arg1: "Receipt", Arg2: "99999"}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no match for wrong kv value")
	}
}

func TestMatchFilter_Chain(t *testing.T) {
	tx := makeTestTx()
	// AND: account matches Electronics AND amount = 47.92
	fn := FilterNode{
		Field: "account", Match: "regex", Arg1: "Electronics",
		Next: []FilterNode{
			{Field: "amount", Match: "exact", Arg1: "47.92"},
		},
	}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for chain: account Electronics AND amount 47.92")
	}
}

func TestMatchFilter_ChainFailsOnSecond(t *testing.T) {
	tx := makeTestTx()
	// AND: account matches Electronics AND amount = 999 → fails second
	fn := FilterNode{
		Field: "account", Match: "regex", Arg1: "Electronics",
		Next: []FilterNode{
			{Field: "amount", Match: "exact", Arg1: "999.00"},
		},
	}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no match for chain with failing second leaf")
	}
}

func TestMatchFilter_Or(t *testing.T) {
	tx := makeTestTx()
	// OR: account matches Food OR amount = 47.92
	fn := FilterNode{
		Next: []FilterNode{
			{Field: "account", Match: "regex", Arg1: "Food"},
			{Field: "amount", Match: "exact", Arg1: "47.92"},
		},
	}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match: OR: Food (no) OR 47.92 (yes)")
	}
}

func TestMatchFilter_OrNoMatch(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{
		Next: []FilterNode{
			{Field: "account", Match: "regex", Arg1: "Food"},
			{Field: "status", Match: "exact", Arg1: "pending"},
		},
	}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no match: OR with both branches failing")
	}
}

func TestMatchFilter_ChainOr(t *testing.T) {
	tx := makeTestTx()
	tx.Tags = map[string]bool{"vacation": true}
	// AND: (account Food OR status clear) AND tag vacation
	// Account Food fails, but status clear is true, tag vacation is true → match
	fn := FilterNode{
		Next: []FilterNode{
			{
				Next: []FilterNode{
					{Field: "account", Match: "regex", Arg1: "Food"},
					{Field: "status", Match: "exact", Arg1: "clear"},
				},
			},
			{Field: "tag", Match: "has", Arg1: "vacation"},
		},
	}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match for chain-or: (Food|clear) AND vacation")
	}
}

func TestMatchFilter_Empty(t *testing.T) {
	tx := makeTestTx()
	fn := FilterNode{}
	ok, err := matchFilter(tx, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected empty filter to match all")
	}
}

// --- QueryWithFilter integration tests ---

func TestQueryWithFilter_Basic(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "test.ledger")
	if err := os.WriteFile(rootPath, []byte(testLedgerMulti), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fn := FilterNode{Field: "account", Match: "regex", Arg1: "Food"}
	results, err := QueryWithFilter(rootPath, fn)
	if err != nil {
		t.Fatalf("QueryWithFilter: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Entry.Description != "Groceries" {
		t.Errorf("got description %q, want Groceries", results[0].Entry.Description)
	}
}

func TestQueryWithFilter_Or(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "test.ledger")
	if err := os.WriteFile(rootPath, []byte(testLedgerMulti), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// OR: Food account OR pending status
	fn := FilterNode{
		Next: []FilterNode{
			{Field: "account", Match: "regex", Arg1: "Food"},
			{Field: "status", Match: "exact", Arg1: "pending"},
		},
	}
	results, err := QueryWithFilter(rootPath, fn)
	if err != nil {
		t.Fatalf("QueryWithFilter: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
}

func TestQueryWithFilter_DateRange(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "test.ledger")
	if err := os.WriteFile(rootPath, []byte(testLedgerMulti), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fn := FilterNode{Field: "date", Match: "range", Arg1: "2025/08/22", Arg2: "2025/08/31"}
	results, err := QueryWithFilter(rootPath, fn)
	if err != nil {
		t.Fatalf("QueryWithFilter: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Entry.Description != "Groceries" {
		t.Errorf("got description %q, want Groceries", results[0].Entry.Description)
	}
}

func TestQueryWithFilter_EmptyFilter(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "test.ledger")
	if err := os.WriteFile(rootPath, []byte(testLedgerMulti), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fn := FilterNode{}
	results, err := QueryWithFilter(rootPath, fn)
	if err != nil {
		t.Fatalf("QueryWithFilter: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
}

func TestQueryWithFilter_Invert(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "test.ledger")
	if err := os.WriteFile(rootPath, []byte(testLedgerMulti), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Exclude anything with Checking account
	fn := FilterNode{Field: "account", Match: "regex", Arg1: "Checking", Invert: true}
	results, err := QueryWithFilter(rootPath, fn)
	if err != nil {
		t.Fatalf("QueryWithFilter: %v", err)
	}
	// Checking is in Amazon and Groceries but not Rent → Rent is returned
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Entry.Description != "Rent" {
		t.Errorf("got description %q, want Rent", results[0].Entry.Description)
	}
}
