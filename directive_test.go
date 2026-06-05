package ledger

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/milochristiansen/ledger/parse/lex"
)

// ---- Directive.String ----

func TestDirective_String(t *testing.T) {
	tests := []struct {
		name string
		d    Directive
		want string
	}{
		{"minimal", Directive{Type: "account", Argument: "Expenses:Food"}, "account Expenses:Food\n"},
		{"one line", Directive{
			Type: "account", Argument: "Expenses:Food", Lines: []string{"note Test Account"},
		}, "account Expenses:Food\n\tnote Test Account\n"},
		{"multiple lines", Directive{
			Type: "commodity", Argument: "USD", Lines: []string{"format $1,000.00", "alias $"},
		}, "commodity USD\n\tformat $1,000.00\n\talias $\n"},
		{"empty argument", Directive{Type: "include", Lines: []string{"other.ledger"}}, "include \n\tother.ledger\n"},
		{"empty type and argument", Directive{Lines: []string{"line1", "line2"}}, " \n\tline1\n\tline2\n"},
		{"empty everything", Directive{}, " \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---- Directive.Compare ----

func TestDirective_Compare(t *testing.T) {
	base := Directive{
		Type: "account", Argument: "Expenses:Food", Lines: []string{"note Test", "alias Food"},
	}

	tests := []struct {
		name string
		a, b Directive
		want bool
	}{
		{"identical", base, base, true},
		{"different type", base, Directive{Type: "payee", Argument: "Expenses:Food", Lines: []string{"note Test", "alias Food"}}, false},
		{"different argument", base, Directive{Type: "account", Argument: "Assets:Cash", Lines: []string{"note Test", "alias Food"}}, false},
		{"different lines length", base, Directive{Type: "account", Argument: "Expenses:Food", Lines: []string{"note Test"}}, false},
		{"different lines content", base, Directive{Type: "account", Argument: "Expenses:Food", Lines: []string{"note Test", "alias Groceries"}}, false},
		{"both empty", Directive{}, Directive{}, true},
		{"empty vs non-empty type", Directive{}, Directive{Type: "account"}, false},
		{"nil lines vs empty lines", Directive{Type: "x", Argument: "y"}, Directive{Type: "x", Argument: "y", Lines: []string{}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Compare(tt.b); got != tt.want {
				t.Errorf("Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---- ErrMalformedAccountName ----

func TestErrMalformedAccountName_Error(t *testing.T) {
	err := ErrMalformedAccountName{Name: "bad name", Location: lex.Location(42)}
	s := err.Error()
	if !strings.Contains(s, "bad name") || !strings.Contains(s, "42") {
		t.Errorf("unexpected error message: %q", s)
	}
}

// ---- File.Accounts ----

func TestFile_Accounts(t *testing.T) {
	f := makeFile

	t.Run("no_directives", func(t *testing.T) {
		accts, err := f().Accounts()
		if err != nil {
			t.Fatal(err)
		}
		if len(accts) != 0 {
			t.Errorf("expected 0 accounts, got %d", len(accts))
		}
	})

	t.Run("skips_non_account", func(t *testing.T) {
		accts, err := f(dir("payee", "SomePayee"), dir("account", "Expenses:Food")).Accounts()
		if err != nil {
			t.Fatal(err)
		}
		if len(accts) != 1 || accts[0].Name != "Expenses:Food" {
			t.Errorf("wrong accounts: %+v", accts)
		}
	})

	t.Run("skips_transactions", func(t *testing.T) {
		accts, err := f(tx("T1"), dir("account", "Expenses:Food")).Accounts()
		if err != nil {
			t.Fatal(err)
		}
		if len(accts) != 1 || accts[0].Name != "Expenses:Food" {
			t.Errorf("wrong accounts: %+v", accts)
		}
	})

	t.Run("subdirectives", func(t *testing.T) {
		accts, err := f(dir("account", "Expenses:Food",
			"default", "alias Groceries", "payee Walmart", "note Food expenses",
		)).Accounts()
		if err != nil {
			t.Fatal(err)
		}
		a := accts[0]
		if !a.Default {
			t.Error("expected Default=true")
		}
		if len(a.Aliases) != 1 || a.Aliases[0] != "Groceries" {
			t.Errorf("wrong aliases: %v", a.Aliases)
		}
		if len(a.Payees) != 1 || a.Payees[0] != "Walmart" {
			t.Errorf("wrong payees: %v", a.Payees)
		}
		if a.Note != "Food expenses" {
			t.Errorf("wrong note: %q", a.Note)
		}
	})

	t.Run("malformed_name_double_space", func(t *testing.T) {
		_, err := f(dir("account", "Expenses:Bad  Name")).Accounts()
		if err == nil {
			t.Error("expected error for double space")
		}
	})

	t.Run("malformed_name_semicolon", func(t *testing.T) {
		_, err := f(dir("account", "Expenses:Bad;Name")).Accounts()
		if err == nil {
			t.Error("expected error for semicolon")
		}
	})

	t.Run("malformed_alias", func(t *testing.T) {
		_, err := f(dir("account", "Expenses:Food", "alias Bad  Alias")).Accounts()
		if err == nil {
			t.Error("expected error for malformed alias")
		}
	})
}

// ---- File.Payees ----

func TestFile_Payees(t *testing.T) {
	f := makeFile

	t.Run("no_directives", func(t *testing.T) {
		payees, err := f().Payees()
		if err != nil {
			t.Fatal(err)
		}
		if len(payees) != 0 {
			t.Errorf("expected 0 payees, got %d", len(payees))
		}
	})

	t.Run("skips_non_account", func(t *testing.T) {
		payees, err := f(dir("commodity", "USD"), dir("account", "Expenses:Food")).Payees()
		if err != nil {
			t.Fatal(err)
		}
		if len(payees) != 1 || payees[0].Name != "Expenses:Food" {
			t.Errorf("wrong payees: %+v", payees)
		}
	})

	t.Run("skips_transactions", func(t *testing.T) {
		payees, err := f(tx("X"), dir("account", "Expenses:Food")).Payees()
		if err != nil {
			t.Fatal(err)
		}
		if len(payees) != 1 || payees[0].Name != "Expenses:Food" {
			t.Errorf("wrong payees: %+v", payees)
		}
	})

	t.Run("with_aliases_and_uuids", func(t *testing.T) {
		payees, err := f(dir("account", "Expenses:Food",
			"alias Walmart", "uuid 1234-5678", "uuid abcd-efgh",
		)).Payees()
		if err != nil {
			t.Fatal(err)
		}
		p := payees[0]
		if p.Name != "Expenses:Food" {
			t.Errorf("wrong name: %q", p.Name)
		}
		if len(p.Aliases) != 1 || p.Aliases[0] != "Walmart" {
			t.Errorf("wrong aliases: %v", p.Aliases)
		}
		if len(p.Uuids) != 2 || p.Uuids[0] != "1234-5678" || p.Uuids[1] != "abcd-efgh" {
			t.Errorf("wrong uuids: %v", p.Uuids)
		}
	})
}

func TestFile_Accounts_CachesParsed(t *testing.T) {
	f := makeFile(dir("account", "Expenses:Food", "payee Walmart"))
	_, err := f.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	accts, err := f.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accts) != 1 || accts[0].Name != "Expenses:Food" {
		t.Error("cached result mismatch")
	}
}

func TestFile_Payees_CachesParsed(t *testing.T) {
	f := makeFile(dir("account", "Expenses:Food", "uuid 1234"))
	_, err := f.Payees()
	if err != nil {
		t.Fatal(err)
	}
	payees, err := f.Payees()
	if err != nil {
		t.Fatal(err)
	}
	if len(payees) != 1 || payees[0].Name != "Expenses:Food" {
		t.Error("cached result mismatch")
	}
}

func TestFile_Includes(t *testing.T) {
	load := func(path string) (*File, error) {
		return &File{}, nil
	}
	f := makeFile(dir("include", "other.ledger"))
	_, err := f.Includes(load)
	if err != nil {
		t.Fatal(err)
	}
	d := f.Entries[0].(*Directive)
	pi, ok := d.Parsed.(*Include)
	if !ok {
		t.Fatal("expected Include in Parsed")
	}
	if pi.Path != "other.ledger" {
		t.Errorf("Path = %q", pi.Path)
	}
	if pi.File == nil {
		t.Error("expected non-nil File")
	}
}

func TestFile_Includes_Error(t *testing.T) {
	errSentinel := errors.New("load error")
	load := func(path string) (*File, error) {
		return nil, errSentinel
	}
	f := makeFile(dir("include", "bad.ledger"))
	_, err := f.Includes(load)
	if err == nil {
		t.Error("expected error")
	}
	pi := f.Entries[0].(*Directive).Parsed.(*Include)
	if pi.Err != errSentinel {
		t.Errorf("Err = %v", pi.Err)
	}
}

// ---- Include-aware methods ----

func TestFile_Transactions_DescendsIncludes(t *testing.T) {
	inner := makeFile(tx("FromInclude"))
	outer := makeFile(
		tx("FromOuter"),
		dir("include", "inner.ledger"),
	)
	outer.Entries[1].(*Directive).Parsed = &Include{Path: "inner.ledger", File: inner}

	ts := outer.Transactions()
	if len(ts) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(ts))
	}
	if ts[0].Description != "FromOuter" || ts[1].Description != "FromInclude" {
		t.Errorf("wrong order: %+v", ts)
	}
}

func TestFile_Directives_DescendsIncludes(t *testing.T) {
	inner := makeFile(dir("account", "Inner"))
	outer := makeFile(
		dir("account", "Outer"),
		dir("include", "inner.ledger"),
	)
	outer.Entries[1].(*Directive).Parsed = &Include{Path: "inner.ledger", File: inner}

	ds := outer.Directives()
	if len(ds) != 2 {
		t.Fatalf("expected 2 directives, got %d", len(ds))
	}
	if ds[0].Argument != "Outer" || ds[1].Argument != "Inner" {
		t.Errorf("wrong order: %+v", ds)
	}
}

func TestFile_Accounts_DescendsIncludes(t *testing.T) {
	inner := makeFile(dir("account", "Inner"))
	outer := makeFile(
		dir("account", "Outer"),
		dir("include", "inner.ledger"),
	)
	outer.Entries[1].(*Directive).Parsed = &Include{Path: "inner.ledger", File: inner}

	accts, err := outer.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accts))
	}
	if accts[0].Name != "Outer" || accts[1].Name != "Inner" {
		t.Errorf("wrong order: %+v", accts)
	}
}

func TestFile_Format_DoesNotDescendIncludes(t *testing.T) {
	inner := makeFile(tx("Inner"))
	outer := makeFile(
		tx("Outer"),
		dir("include", "inner.ledger"),
	)
	outer.Entries[1].(*Directive).Parsed = &Include{Path: "inner.ledger", File: inner}

	var buf bytes.Buffer
	err := outer.Format(&buf)
	if err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if strings.Contains(s, "Inner") {
		t.Error("Format should not resolve includes")
	}
	if !strings.Contains(s, "Outer") {
		t.Error("Format should contain outer transaction")
	}
}

// Helpers shared with file_test.go
func makeFile(entries ...Entry) *File {
	return &File{Entries: entries}
}

func tx(desc string, postings ...Posting) *Transaction {
	return &Transaction{Description: desc, Postings: postings}
}

func dir(typ, arg string, lines ...string) *Directive {
	return &Directive{Type: typ, Argument: arg, Lines: lines}
}
