package parse_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse"
)

// ---- ParseLedger / ParseLedgerString ----

func TestParseLedgerString(t *testing.T) {
	f, err := parse.ParseLedgerString("")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(f.Entries))
	}
}

func TestParseLedger_Empty(t *testing.T) {
	f, err := parse.ParseLedger(parse.NewCharReader("", 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(f.Entries))
	}
}

func TestParseLedger_CommentOnly(t *testing.T) {
	f, err := parse.ParseLedgerString("; this is a comment\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(f.Entries))
	}
}

func TestParseLedger_BlankLines(t *testing.T) {
	f, err := parse.ParseLedgerString("\n\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(f.Entries))
	}
}

func TestParseLedger_Directive(t *testing.T) {
	input := "account Expenses:Food\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f.Entries))
	}
	d, ok := f.Entries[0].(*ledger.Directive)
	if !ok {
		t.Fatal("expected Directive")
	}
	if d.Type != "account" || d.Argument != "Expenses:Food" {
		t.Errorf("wrong directive: type=%q arg=%q", d.Type, d.Argument)
	}
}

func TestParseLedger_DirectiveWithLines(t *testing.T) {
	input := "account Expenses:Food\n\tnote Test\n\tdefault\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := f.Entries[0].(*ledger.Directive)
	if !ok {
		t.Fatal("expected Directive")
	}
	if len(d.Lines) != 2 || d.Lines[0] != "note Test" || d.Lines[1] != "default" {
		t.Errorf("wrong lines: %q", d.Lines)
	}
}

func TestParseLedger_DirectiveNoArgument(t *testing.T) {
	input := "include\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := f.Entries[0].(*ledger.Directive)
	if !ok {
		t.Fatal("expected Directive")
	}
	if d.Argument != "" {
		t.Errorf("expected empty argument, got %q", d.Argument)
	}
}

func TestParseLedger_BasicTransaction(t *testing.T) {
	input := "2021/09/25 Test Desc\n\tExpenses:Foo  $20.00\n\tAssets:Bar\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f.Entries))
	}
	tr, ok := f.Entries[0].(*ledger.Transaction)
	if !ok {
		t.Fatal("expected Transaction")
	}
	if tr.Description != "Test Desc" {
		t.Errorf("description = %q", tr.Description)
	}
	if tr.Date.Year() != 2021 || tr.Date.Month() != 9 || tr.Date.Day() != 25 {
		t.Errorf("date = %v", tr.Date)
	}
	if len(tr.Postings) != 2 {
		t.Fatalf("expected 2 postings, got %d", len(tr.Postings))
	}
	if tr.Postings[0].Account != "Expenses:Foo" || tr.Postings[0].Value != 200000 {
		t.Errorf("posting 0: acct=%q val=%d", tr.Postings[0].Account, tr.Postings[0].Value)
	}
	if !tr.Postings[1].Null {
		t.Error("posting 1 should be null")
	}
}

func TestParseLedger_WithClearDate(t *testing.T) {
	input := "2021/09/25=2021/09/30 Test\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.ClearDate.IsZero() {
		t.Error("expected clear date")
	}
	if tr.ClearDate.Day() != 30 {
		t.Errorf("clear date day = %d", tr.ClearDate.Day())
	}
}

func TestParseLedger_StatusClear(t *testing.T) {
	input := "2021/09/25 * Test\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.Status != ledger.StatusClear {
		t.Errorf("status = %v", tr.Status)
	}
}

func TestParseLedger_StatusPending(t *testing.T) {
	input := "2021/09/25 ! Test\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.Status != ledger.StatusPending {
		t.Errorf("status = %v", tr.Status)
	}
}

func TestParseLedger_WithCode(t *testing.T) {
	input := "2021/09/25 (PAY) Test\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.Code != "PAY" {
		t.Errorf("code = %q", tr.Code)
	}
}

func TestParseLedger_WithTags(t *testing.T) {
	input := "2021/09/25 Test\n\t; :Tag1:Tag2:\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if !tr.Tags["Tag1"] || !tr.Tags["Tag2"] {
		t.Errorf("tags = %v", tr.Tags)
	}
}

func TestParseLedger_WithKVPair(t *testing.T) {
	input := "2021/09/25 Test\n\t; Key: Value\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.KVPairs["Key"] != "Value" {
		t.Errorf("KVPairs = %v", tr.KVPairs)
	}
}

func TestParseLedger_WithComment(t *testing.T) {
	input := "2021/09/25 Test\n\t; just a comment\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if len(tr.Comments) != 1 || tr.Comments[0] != "just a comment" {
		t.Errorf("comments = %q", tr.Comments)
	}
}

func TestParseLedger_PostingWithStatus(t *testing.T) {
	input := "2021/09/25 Test\n\t* A  $10.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.Postings[0].Status != ledger.StatusClear {
		t.Errorf("posting status = %v", tr.Postings[0].Status)
	}
}

func TestParseLedger_PostingWithAssert(t *testing.T) {
	input := "2021/09/25 Test\n\tA  $10.00 = $10.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if !tr.Postings[0].HasAssert || tr.Postings[0].Assert != 100000 {
		t.Errorf("posting assert = %v / %v", tr.Postings[0].HasAssert, tr.Postings[0].Assert)
	}
}

func TestParseLedger_PostingWithNote(t *testing.T) {
	input := "2021/09/25 Test\n\tA  $10.00 ; hello\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.Postings[0].Note != "hello" {
		t.Errorf("note = %q", tr.Postings[0].Note)
	}
}

func TestParseLedger_PostingWithSpaceAccount(t *testing.T) {
	input := "2021/09/25 Test\n\tExpenses:Food Groceries  $10.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.Postings[0].Account != "Expenses:Food Groceries" {
		t.Errorf("account = %q", tr.Postings[0].Account)
	}
}

// ---- ParseLedger Error Cases ----

func TestParseLedger_MalformedDate(t *testing.T) {
	_, err := parse.ParseLedgerString("2021/1/25 Test\n\tA  $1.00\n\tB\n")
	if err == nil {
		t.Error("expected error")
	}
}

func TestParseLedger_MalformedTagLine(t *testing.T) {
	_, err := parse.ParseLedgerString("2021/09/25 Test\n\t; :X:Y: not allowed after tags\n\tA  $1.00\n\tB\n")
	if err == nil {
		t.Error("expected error for malformed tag line")
	}
}

func TestParseLedger_MalformedPostingLine(t *testing.T) {
	_, err := parse.ParseLedgerString("2021/09/25 Test\n\tA  $10.00 X\n\tB\n")
	if err == nil {
		t.Error("expected error")
	}
}

func TestParseLedger_UnexpectedEOF_AfterDate(t *testing.T) {
	_, err := parse.ParseLedgerString("2021/09/25")
	if err == nil {
		t.Error("expected unexpected EOF error")
	}
}

func TestParseLedger_UnexpectedEOF_DuringPosting(t *testing.T) {
	_, err := parse.ParseLedgerString("2021/09/25 Test\n\tExpenses:Foo")
	if err == nil {
		t.Error("expected unexpected EOF error")
	}
}

func TestParseLedger_CodeNewline(t *testing.T) {
	_, err := parse.ParseLedgerString("2021/09/25 (code\n Test\n\tA  $1.00\n\tB\n")
	if err == nil {
		t.Error("expected malformed error for newline in code")
	}
}

func TestParseLedger_NullAssert(t *testing.T) {
	_, err := parse.ParseLedgerString("2021/09/25 Test\n\tA  $10.00 = \n\tB\n")
	if err == nil {
		t.Error("expected error for null balance assertion")
	}
}

func TestParseLedger_CodeTrailingWhitespace(t *testing.T) {
	input := "2021/09/25 (PAY)  Test\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.Code != "PAY" {
		t.Errorf("code = %q", tr.Code)
	}
}

func TestParseLedger_PostingPendingStatus(t *testing.T) {
	input := "2021/09/25 Test\n\t! A  $10.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.Postings[0].Status != ledger.StatusPending {
		t.Errorf("posting status = %v", tr.Postings[0].Status)
	}
}

func TestParseLedger_CommentState4_ColonNoSpace(t *testing.T) {
	input := "2021/09/25 Test\n\t; key:no-space\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if len(tr.Comments) != 1 || tr.Comments[0] != "key:no-space" {
		t.Errorf("comments = %q", tr.Comments)
	}
}

func TestParseLedger_CommentState4_KeyWhitespace(t *testing.T) {
	input := "2021/09/25 Test\n\t; key space: value\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if len(tr.Comments) != 1 || tr.Comments[0] != "key space: value" {
		t.Errorf("comments = %q", tr.Comments)
	}
}

func TestParseLedger_WithClearDateAndStatus(t *testing.T) {
	input := "2021/09/25=2021/09/28 * Test\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.ClearDate.Day() != 28 || tr.Status != ledger.StatusClear {
		t.Errorf("clear=%v status=%v", tr.ClearDate, tr.Status)
	}
}

func TestParseLedger_PendingStatusWithCode(t *testing.T) {
	input := "2021/09/25 ! (PAY) Test\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.Status != ledger.StatusPending || tr.Code != "PAY" {
		t.Errorf("status=%v code=%q", tr.Status, tr.Code)
	}
}

func TestParseLedger_KVPairWhitespaceInValue(t *testing.T) {
	input := "2021/09/25 Test\n\t; Key: Value with spaces\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if tr.KVPairs["Key"] != "Value with spaces" {
		t.Errorf("KVPairs = %v", tr.KVPairs)
	}
}

func TestParseLedger_EmptyTagDiscarded(t *testing.T) {
	input := "2021/09/25 Test\n\t; ::Tag:\n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if !tr.Tags["Tag"] || tr.Tags[""] {
		t.Errorf("tags = %v", tr.Tags)
	}
}

func TestParseLedger_TagWithTrailingSpaces(t *testing.T) {
	input := "2021/09/25 Test\n\t; :Tag:  \n\tA  $1.00\n\tB\n"
	f, err := parse.ParseLedgerString(input)
	if err != nil {
		t.Fatal(err)
	}
	tr := f.Entries[0].(*ledger.Transaction)
	if !tr.Tags["Tag"] {
		t.Errorf("tags = %v", tr.Tags)
	}
}


// ---- ReadAmount ----

func TestReadAmount(t *testing.T) {
	tests := []struct {
		input    string
		want     int64
		wantNull bool
	}{
		{"$20.00\n", 200000, false},
		{"20.00\n", 200000, false},
		{"$20\n", 200000, false},
		{"20\n", 200000, false},
		{"-20.00\n", -200000, false},
		{"-20\n", -200000, false},
		{"$1,000.00\n", 10000000, false},
		{"$0.01\n", 100, false},
		{"$0.99\n", 9900, false},
		{"$0.005\n", 50, false},
		{"$0.055\n", 550, false},
		{"\n", 0, true},
	}
	for _, tt := range tests {
		v, null, err := parse.ReadAmount(parse.NewCharReader(tt.input, 1))
		if err != nil {
			t.Errorf("ReadAmount(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if null != tt.wantNull {
			t.Errorf("ReadAmount(%q) null=%v, want %v", tt.input, null, tt.wantNull)
		}
		if v != tt.want {
			t.Errorf("ReadAmount(%q) = %d, want %d", tt.input, v, tt.want)
		}
	}
}

func TestReadAmount_Errors(t *testing.T) {
	tests := []string{
		"$.\n",         // leading decimal
		"$0.0.0\n",     // multiple decimals
		"$0.00000\n",   // too many fractional digits
	}
	for _, input := range tests {
		_, _, err := parse.ReadAmount(parse.NewCharReader(input, 1))
		if err == nil {
			t.Errorf("ReadAmount(%q) expected error", input)
		}
	}
}

func TestReadAmount_EOF(t *testing.T) {
	_, _, err := parse.ReadAmount(parse.NewCharReader("$", 1))
	if err == nil {
		t.Error("expected EOF error")
	}
}

func TestReadAmount_Pennies(t *testing.T) {
	for a := 0; a < 10; a++ {
		for b := 0; b < 10; b++ {
			check := int64(20000 + (a * 1000) + (b * 100))
			input := string([]rune{rune('0' + a), rune('0' + b)})

			v, _, err := parse.ReadAmount(parse.NewCharReader("$2."+input+"\n", 0))
			if err != nil {
				t.Error(input, err)
			}
			if v != check {
				t.Error("$2."+input, v, "!=", check)
			}
		}
	}
}

// ---- ReadUntilTrimmed ----
func TestReadUntilTrimmed_LeadingWhitespace(t *testing.T) {
	cr := parse.NewCharReader("  abc\n", 1)
	result, err := parse.ReadUntilTrimmed(cr, "\n")
	if err != nil {
		t.Fatal(err)
	}
	if result != "abc" {
		t.Errorf("expected %q, got %q", "abc", result)
	}
}

func TestReadUntilTrimmed_TrailingWhitespace(t *testing.T) {
	cr := parse.NewCharReader("abc  \n", 1)
	result, err := parse.ReadUntilTrimmed(cr, "\n")
	if err != nil {
		t.Fatal(err)
	}
	if result != "abc" {
		t.Errorf("expected %q, got %q", "abc", result)
	}
}

func TestReadUntilTrimmed_BothEndsWhitespace(t *testing.T) {
	cr := parse.NewCharReader("  abc  \n", 1)
	result, err := parse.ReadUntilTrimmed(cr, "\n")
	if err != nil {
		t.Fatal(err)
	}
	if result != "abc" {
		t.Errorf("expected %q, got %q", "abc", result)
	}
}

func TestReadUntilTrimmed_SingleSpace(t *testing.T) {
	cr := parse.NewCharReader(" \n", 1)
	result, err := parse.ReadUntilTrimmed(cr, "\n")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestReadUntilTrimmed_OnlyWhitespace(t *testing.T) {
	cr := parse.NewCharReader("  \t \n", 1)
	result, err := parse.ReadUntilTrimmed(cr, "\n")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestReadUntilTrimmed_NoWhitespace(t *testing.T) {
	cr := parse.NewCharReader("abc\n", 1)
	result, err := parse.ReadUntilTrimmed(cr, "\n")
	if err != nil {
		t.Fatal(err)
	}
	if result != "abc" {
		t.Errorf("expected %q, got %q", "abc", result)
	}
}

func TestReadUntilTrimmed_TrailingTab(t *testing.T) {
	cr := parse.NewCharReader("abc\t\n", 1)
	result, err := parse.ReadUntilTrimmed(cr, "\n")
	if err != nil {
		t.Fatal(err)
	}
	if result != "abc" {
		t.Errorf("expected %q, got %q", "abc", result)
	}
}

func TestReadUntilTrimmed_SingleTab(t *testing.T) {
	cr := parse.NewCharReader("\t\n", 1)
	result, err := parse.ReadUntilTrimmed(cr, "\n")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// ---- ParseDate ----

func TestParseDate(t *testing.T) {
	tests := []struct {
		input string
		year  int
		month time.Month
		day   int
	}{
		{"2021/09/25\n", 2021, 9, 25},
		{"2021-09-25\n", 2021, 9, 25},
		{"2021.09.25\n", 2021, 9, 25},
		{"2021/01/01\n", 2021, 1, 1},
		{"2021/12/31\n", 2021, 12, 31},
		{"1999/06/15\n", 1999, 6, 15},
	}

	for _, tt := range tests {
		cr := parse.NewCharReader(tt.input, 1)
		dt, err := parse.ParseDate(cr)
		if err != nil {
			t.Errorf("ParseDate(%q) error: %v", tt.input, err)
			continue
		}
		if dt.Year() != tt.year || dt.Month() != tt.month || dt.Day() != tt.day {
			t.Errorf("ParseDate(%q) = %v, want %d/%d/%d",
				tt.input, dt, tt.year, tt.month, tt.day)
		}
	}
}

func TestParseDate_Errors(t *testing.T) {
	tests := []string{
		"abc\n",         // not a date
		"2021/1/25\n",   // month not 2 digits
		"2021/09/2\n",   // day not 2 digits
		"20/09/25\n",    // year too short
		"2021:09:25\n",  // wrong separator
		"2021/09\n",     // incomplete (not enough chars for month)
	}

	for _, input := range tests {
		_, err := parse.ParseDate(parse.NewCharReader(input, 1))
		if err == nil {
			t.Errorf("ParseDate(%q) expected error", input)
		}
	}
}

func TestParseDate_EOF(t *testing.T) {
	_, err := parse.ParseDate(parse.NewCharReader("2021", 1))
	if err == nil {
		t.Error("expected EOF error")
	}
}

// ---- NewCharReader / NewRawCharReader ----

func TestNewCharReader(t *testing.T) {
	cr := parse.NewCharReader("a", 5)
	if cr.C != 'a' {
		t.Errorf("C = %q", cr.C)
	}
}

func TestNewRawCharReader(t *testing.T) {
	cr := parse.NewRawCharReader(strings.NewReader("x"), 10)
	if cr.C != 'x' {
		t.Errorf("C = %q", cr.C)
	}
}

// ---- Error types ----

func TestErrBadDate_Error(t *testing.T) {
	err := parse.ErrBadDate(parse.NewCharReader("", 42).L)
	if err.Error() == "" {
		t.Error("empty error message")
	}
}

func TestErrBadAmount_Error(t *testing.T) {
	err := parse.ErrBadAmount(parse.NewCharReader("", 42).L)
	if err.Error() == "" {
		t.Error("empty error message")
	}
}

func TestErrUnexpectedEnd_Error(t *testing.T) {
	err := parse.ErrUnexpectedEnd(parse.NewCharReader("", 42).L)
	if err.Error() == "" {
		t.Error("empty error message")
	}
}

func TestErrMalformed_Error(t *testing.T) {
	err := parse.ErrMalformed(parse.NewCharReader("", 42).L)
	if err.Error() == "" {
		t.Error("empty error message")
	}
}

func TestErrMalformedTagLine_Error(t *testing.T) {
	err := parse.ErrMalformedTagLine(parse.NewCharReader("", 42).L)
	if err.Error() == "" {
		t.Error("empty error message")
	}
}

// ---- Helper to suppress unused import ----
var _ io.RuneReader = strings.NewReader("")
