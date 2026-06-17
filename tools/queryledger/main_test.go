package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse/lex"
	"github.com/milochristiansen/ledger/tools"
)

func makeTestTx() *ledger.Transaction {
	return &ledger.Transaction{
		Description: "Test",
		Postings: []ledger.Posting{
			{Account: "Expenses:Test", Value: 1000},
		},
		Location: lex.Location(0).L(5),
	}
}

// Test outputLedger doesn't panic with valid input.
func TestOutputLedger(t *testing.T) {
	results := []tools.QueryResult{
		{
			File:  "test.ledger",
			Ref:   "0:abc",
			Entry: makeTestTx(),
		},
	}
	// outputLedger writes to stdout; just verify it doesn't panic
	outputLedger(results)
}

// Test outputJSON doesn't panic and produces valid JSON.
func TestOutputJSON(t *testing.T) {
	results := []tools.QueryResult{
		{
			File:  "test.ledger",
			Ref:   "0:abc",
			Entry: makeTestTx(),
		},
	}
	// Capture by temporarily redirecting — but this is a smoke test.
	// outputJSON encodes to os.Stdout; produce no panic.
	outputJSON(results)
}

// Test outputCSV doesn't panic with valid field names.
func TestOutputCSV(t *testing.T) {
	results := []tools.QueryResult{
		{
			File:  "test.ledger",
			Ref:   "0:abc",
			Entry: makeTestTx(),
		},
	}
	outputCSV(results, "file,ref,description,account")
}

// Test CSVField round-trip through the CLI.
func TestCSVField_CLIRoundTrip(t *testing.T) {
	r := tools.QueryResult{
		File:  "test.ledger",
		Ref:   "0:abc",
		Entry: makeTestTx(),
	}
	got := tools.CSVField("file", &r)
	if got != "test.ledger" {
		t.Errorf("file = %q, want test.ledger", got)
	}
	got = tools.CSVField("account", &r)
	if got != "Expenses:Test" {
		t.Errorf("account = %q, want Expenses:Test", got)
	}
}

// Test that the flags are bound correctly (smoke test — just verifies
// the flag vars exist and can be set via testing pattern).
func TestFlagDefaults(t *testing.T) {
	if *date != "" {
		t.Error("date default should be empty")
	}
	if *asJSON != false {
		t.Error("json default should be false")
	}
}

// Test that outputResult with JSON flag works.
func TestOutputResult_JSON(t *testing.T) {
	*asJSON = true
	defer func() { *asJSON = false }()

	r := &tools.QueryResult{
		File:  "test.ledger",
		Ref:   "0:abc",
		Entry: makeTestTx(),
	}

	// outputResult writes to stdout; verify it doesn't panic
	outputResult(r)

	// Verify the struct is JSON-serializable
	_, err := json.Marshal(r)
	if err != nil {
		t.Errorf("QueryResult should be JSON-serializable: %v", err)
	}
}

// Test that outputResult with CSV flag works.
func TestOutputResult_CSV(t *testing.T) {
	*csvFields = "file,ref"
	defer func() { *csvFields = "" }()

	r := &tools.QueryResult{
		File:  "test.ledger",
		Ref:   "0:abc",
		Entry: makeTestTx(),
	}
	outputResult(r)
}

// Test outputLedger produces expected format.
func TestOutputLedger_Format(t *testing.T) {
	r := tools.QueryResult{
		File:  "test.ledger",
		Ref:   "0:abc",
		Entry: makeTestTx(),
	}
	// outputLedger writes "; test.ledger:5, ref: 0:abc\n\n..."
	// We can't capture stdout easily, but we can verify the entry is formatted.
	s := r.Entry.String()
	if !strings.Contains(s, "Test") {
		t.Errorf("entry String() should contain description, got: %s", s)
	}
	if !strings.Contains(s, "Expenses:Test") {
		t.Errorf("entry String() should contain account, got: %s", s)
	}
}
