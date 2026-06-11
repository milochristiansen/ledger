package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse/lex"
)

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
			{Account: "Expenses:Electronics", Value: 47924, Null: false, Status: ledger.StatusClear},
			{Account: "Assets:Checking", Value: -47924, Null: false, HasAssert: true, Assert: 100000, Note: "Credit card charge"},
		},
		Location: lex.Location(0).L(42),
	}
}

func TestFieldValue(t *testing.T) {
	tx := makeTestTx()

	tests := []struct {
		field   string
		fileArg string
		refArg  string
		want    string
	}{
		{"file", "2025.ledger", "", "2025.ledger"},
		{"line", "", "", "42"},
		{"ref", "", "223:c2bba691", "223:c2bba691"},
		{"date", "", "", "2025/08/20"},
		{"clear_date", "", "", "2025/08/22"},
		{"status", "", "", "*"},
		{"code", "", "", "TX123"},
		{"description", "", "", "Amazon"},
		{"account", "", "", "Expenses:Electronics"},
		{"account:0", "", "", "Expenses:Electronics"},
		{"account:1", "", "", "Assets:Checking"},
		{"account:2", "", "", ""},
		{"amount", "", "", "4.79"},
		{"amount:0", "", "", "4.79"},
		{"amount:1", "", "", "-4.79"},
		{"note", "", "", ""},
		{"note:0", "", "", ""},
		{"note:1", "", "", "Credit card charge"},
		{"note:2", "", "", ""},
		{"status:0", "", "", "*"},
		{"status:1", "", "", ""},
		{"status:2", "", "", ""},
		{"assert", "", "", ""},
		{"assert:0", "", "", ""},
		{"assert:1", "", "", "10.00"},
		{"assert:2", "", "", ""},
		{"unknown", "", "", ""},
	}
	for _, tt := range tests {
		got := fieldValue(tt.field, tt.fileArg, tt.refArg, tx)
		if got != tt.want {
			t.Errorf("fieldValue(%q) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestMakeRef(t *testing.T) {
	tx := makeTestTx()
	ref := makeRef("2025.ledger", 5, tx)
	if ref == "" {
		t.Error("makeRef returned empty")
	}
	ref2 := makeRef("2025.ledger", 5, tx)
	if ref != ref2 {
		t.Errorf("makeRef not deterministic: %q vs %q", ref, ref2)
	}
	ref3 := makeRef("2025.ledger", 6, tx)
	if ref == ref3 {
		t.Error("makeRef should differ by index")
	}
}

// ---- filesystem tests ----

const testLedger = `2025/08/20 Amazon
    Expenses:Electronics    $47.92
    Assets:Checking        -$47.92

2025/08/22 * Groceries
    Expenses:Food           $12.50
    Assets:Checking        -$12.50

2025/09/01 ! Rent
    Expenses:Rent           $800.00
    Assets:Savings         -$800.00
`

func writeTempLedger(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func resetQueryFlags() {
	*date = ""
	*acct = ""
	*excludeAcct = ""
	*payee = ""
	*excludePayee = ""
	*statusFl = ""
	*excludeStFl = ""
	*amount = ""
}

func TestQuery_DateFilter(t *testing.T) {
	defer resetQueryFlags()
	*date = "2025/08/01:2025/08/31"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (Aug), got %d", len(results))
	}
}

func TestQuery_AccountFilter(t *testing.T) {
	defer resetQueryFlags()
	*acct = "Food"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
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
	defer resetQueryFlags()
	*payee = "Amazon"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
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
	defer resetQueryFlags()
	*amount = "800.00"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
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
	defer resetQueryFlags()
	*amount = "10.00:50.00"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
	if err != nil {
		t.Fatal(err)
	}
	// 47.92 and 12.50 are in range; 800.00 is not
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestQuery_ExcludeAccount(t *testing.T) {
	defer resetQueryFlags()
	*excludeAcct = "Checking"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
	if err != nil {
		t.Fatal(err)
	}
	// Rent uses Savings, not Checking
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Description != "Rent" {
		t.Errorf("expected Rent, got %s", results[0].Entry.Description)
	}
}

func TestQuery_ExcludePayee(t *testing.T) {
	defer resetQueryFlags()
	*excludePayee = "Amazon"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (Groceries + Rent), got %d", len(results))
	}
}

func TestQuery_AllFilters(t *testing.T) {
	defer resetQueryFlags()
	*date = "2025/08/01:2025/08/31"
	*acct = "Food"
	*amount = "10.00:20.00"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
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
	defer resetQueryFlags()
	*acct = "Nonexistent"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestQuery_StatusClear(t *testing.T) {
	defer resetQueryFlags()
	*statusFl = "clear"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
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
	defer resetQueryFlags()
	*statusFl = "!"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
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
	defer resetQueryFlags()
	*statusFl = "none"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
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
	defer resetQueryFlags()
	*excludeStFl = "clear"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (excluding clear), got %d", len(results))
	}
}

func TestQuery_StatusInvalid(t *testing.T) {
	defer resetQueryFlags()
	*statusFl = "bogus"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	_, err := query(filepath.Join(dir, "test.ledger"))
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
}

func TestFindByRef_Found(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedger)

	// Get a known ref by querying first
	*date = "2025/08/01:2025/08/31"
	*acct = ""
	*excludeAcct = ""
	*payee = ""
	*excludePayee = ""
	*amount = ""
	results, err := query(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results from query")
	}
	resetQueryFlags()

	result, err := findByRef(path, results[0].Ref, "")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("findByRef returned nil")
	}
	if result.Entry.Description != results[0].Entry.Description {
		t.Errorf("wrong transaction: %s vs %s", result.Entry.Description, results[0].Entry.Description)
	}
}

func TestFindByRef_NotFound(t *testing.T) {
	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	result, err := findByRef(filepath.Join(dir, "test.ledger"), "999:deadbeef", "")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil for unknown ref")
	}
}

func TestFindByRef_WithScopeFile(t *testing.T) {
	dir := t.TempDir()
	path := writeTempLedger(t, dir, "test.ledger", testLedger)

	// Get a known ref
	*date = "2025/08/01:2025/08/31"
	*acct = ""
	*excludeAcct = ""
	*payee = ""
	*excludePayee = ""
	*amount = ""
	results, err := query(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results from query")
	}
	resetQueryFlags()

	// Look up in wrong file → not found
	result, err := findByRef(path, results[0].Ref, "nonexistent.ledger")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil when scoped to wrong file")
	}

	// Look up in correct file → found
	result, err = findByRef(path, results[0].Ref, "test.ledger")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Error("expected result when scoped to correct file")
	}

	// Look up in correct file but with ref not in that file → not found
	result, err = findByRef(path, "999:deadbeef", "test.ledger")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil when ref not found in scoped file")
	}
}

func TestQuery_AmountRangeSwap(t *testing.T) {
	defer resetQueryFlags()
	*amount = "50.00:10.00"

	dir := t.TempDir()
	writeTempLedger(t, dir, "test.ledger", testLedger)

	results, err := query(filepath.Join(dir, "test.ledger"))
	if err != nil {
		t.Fatal(err)
	}
	// 47.92 and 12.50 are within 10.00-50.00 after swap; 800.00 is not
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestFieldValue_EdgeCases(t *testing.T) {
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
		got := fieldValue("clear_date", "", "", tx)
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
		got := fieldValue("amount", "", "", tx)
		if got != "" {
			t.Errorf("expected empty string for null posting, got %q", got)
		}
	})

	t.Run("account_oob", func(t *testing.T) {
		tx := makeTestTx()
		got := fieldValue("account:5", "", "", tx)
		if got != "" {
			t.Errorf("expected empty string for out-of-bounds account index, got %q", got)
		}
	})

	t.Run("assert_no_assert", func(t *testing.T) {
		tx := makeTestTx()
		got := fieldValue("assert:0", "", "", tx)
		if got != "" {
			t.Errorf("expected empty string for posting without assert, got %q", got)
		}
	})
}

// ---- multi-file (include directive) tests ----

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
	defer resetQueryFlags()

	dir := t.TempDir()
	writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)

	results, err := query(filepath.Join(dir, "root.ledger"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 transactions (root + sub), got %d", len(results))
	}
}

func TestFindByRef_MultiFile(t *testing.T) {
	dir := t.TempDir()
	rootPath := writeTempLedger(t, dir, "root.ledger", rootWithInclude)
	writeTempLedger(t, dir, "sub.ledger", subLedger)
	// Query everything to get refs
	*date = ""
	*acct = ""
	*excludeAcct = ""
	*payee = ""
	*excludePayee = ""
	*amount = ""
	results, err := query(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	resetQueryFlags()
	if len(results) < 2 {
		t.Fatal("not enough results")
	}

	// Find the sub-ledger transaction by ref (unscoped)
	subResult := results[1] // Groceries from sub.ledger
	result, err := findByRef(rootPath, subResult.Ref, "")
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
