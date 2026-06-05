package ledger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/milochristiansen/ledger/parse/lex"
)

func makeFile(entries ...Entry) *File {
	return &File{Entries: entries}
}

func tx(desc string, postings ...Posting) *Transaction {
	return &Transaction{Description: desc, Postings: postings}
}

func dir(typ, arg string, lines ...string) *Directive {
	return &Directive{Type: typ, Argument: arg, Lines: lines}
}

// ---- Format ----

func TestFile_Format(t *testing.T) {
	t.Run("empty_file", func(t *testing.T) {
		f := makeFile()
		var buf bytes.Buffer
		err := f.Format(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected empty output, got %q", buf.String())
		}
	})

	t.Run("transactions_only", func(t *testing.T) {
		f := makeFile(tx("First"), tx("Second"))
		var buf bytes.Buffer
		err := f.Format(&buf)
		if err != nil {
			t.Fatal(err)
		}
		s := buf.String()
		if !strings.Contains(s, "First") || !strings.Contains(s, "Second") {
			t.Errorf("missing transactions: %q", s)
		}
	})

	t.Run("directives_only", func(t *testing.T) {
		f := makeFile(
			dir("account", "Expenses:Food"),
			dir("account", "Assets:Cash"),
		)
		var buf bytes.Buffer
		err := f.Format(&buf)
		if err != nil {
			t.Fatal(err)
		}
		s := buf.String()
		if !strings.Contains(s, "Expenses:Food") || !strings.Contains(s, "Assets:Cash") {
			t.Errorf("missing directives: %q", s)
		}
	})

	t.Run("interleaved", func(t *testing.T) {
		f := makeFile(
			dir("account", "Before"),
			tx("T1"),
			dir("account", "Middle"),
			tx("T2"),
			dir("account", "After"),
		)
		var buf bytes.Buffer
		err := f.Format(&buf)
		if err != nil {
			t.Fatal(err)
		}
		s := buf.String()
		before := strings.Index(s, "Before")
		t1 := strings.Index(s, "T1")
		middle := strings.Index(s, "Middle")
		t2 := strings.Index(s, "T2")
		after := strings.Index(s, "After")
		if !(before < t1 && t1 < middle && middle < t2 && t2 < after) {
			t.Errorf("wrong interleave order: %q", s)
		}
	})
}

// ---- ErrMalformedAccountName ----

func TestErrMalformedAccountName_Error(t *testing.T) {
	err := ErrMalformedAccountName{Name: "bad name", Location: lex.Location(42)}
	s := err.Error()
	if !strings.Contains(s, "bad name") || !strings.Contains(s, "42") {
		t.Errorf("unexpected error message: %q", s)
	}
}

// ---- Accounts ----

func TestFile_Accounts(t *testing.T) {
	t.Run("no_directives", func(t *testing.T) {
		f := makeFile()
		accts, err := f.Accounts()
		if err != nil {
			t.Fatal(err)
		}
		if len(accts) != 0 {
			t.Errorf("expected 0 accounts, got %d", len(accts))
		}
	})

	t.Run("skips_non_account", func(t *testing.T) {
		f := makeFile(
			dir("payee", "SomePayee"),
			dir("account", "Expenses:Food"),
		)
		accts, err := f.Accounts()
		if err != nil {
			t.Fatal(err)
		}
		if len(accts) != 1 || accts[0].Name != "Expenses:Food" {
			t.Errorf("wrong accounts: %+v", accts)
		}
	})

	t.Run("skips_transactions", func(t *testing.T) {
		f := makeFile(
			tx("T1"),
			dir("account", "Expenses:Food"),
		)
		accts, err := f.Accounts()
		if err != nil {
			t.Fatal(err)
		}
		if len(accts) != 1 || accts[0].Name != "Expenses:Food" {
			t.Errorf("wrong accounts: %+v", accts)
		}
	})

	t.Run("subdirectives", func(t *testing.T) {
		f := makeFile(dir("account", "Expenses:Food",
			"default",
			"alias Groceries",
			"payee Walmart",
			"note Food expenses",
		))
		accts, err := f.Accounts()
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
		f := makeFile(dir("account", "Expenses:Bad  Name"))
		_, err := f.Accounts()
		if err == nil {
			t.Error("expected error for double space")
		}
	})

	t.Run("malformed_name_semicolon", func(t *testing.T) {
		f := makeFile(dir("account", "Expenses:Bad;Name"))
		_, err := f.Accounts()
		if err == nil {
			t.Error("expected error for semicolon")
		}
	})

	t.Run("malformed_alias", func(t *testing.T) {
		f := makeFile(dir("account", "Expenses:Food", "alias Bad  Alias"))
		_, err := f.Accounts()
		if err == nil {
			t.Error("expected error for malformed alias")
		}
	})
}

// ---- Payees ----

func TestFile_Payees(t *testing.T) {
	t.Run("no_directives", func(t *testing.T) {
		f := makeFile()
		payees, err := f.Payees()
		if err != nil {
			t.Fatal(err)
		}
		if len(payees) != 0 {
			t.Errorf("expected 0 payees, got %d", len(payees))
		}
	})

	t.Run("skips_non_account", func(t *testing.T) {
		f := makeFile(
			dir("commodity", "USD"),
			dir("account", "Expenses:Food"),
		)
		payees, err := f.Payees()
		if err != nil {
			t.Fatal(err)
		}
		if len(payees) != 1 || payees[0].Name != "Expenses:Food" {
			t.Errorf("wrong payees: %+v", payees)
		}
	})

	t.Run("skips_transactions", func(t *testing.T) {
		f := makeFile(
			tx("X"),
			dir("account", "Expenses:Food"),
		)
		payees, err := f.Payees()
		if err != nil {
			t.Fatal(err)
		}
		if len(payees) != 1 || payees[0].Name != "Expenses:Food" {
			t.Errorf("wrong payees: %+v", payees)
		}
	})

	t.Run("with_aliases_and_uuids", func(t *testing.T) {
		f := makeFile(dir("account", "Expenses:Food",
			"alias Walmart",
			"uuid 1234-5678",
			"uuid abcd-efgh",
		))
		payees, err := f.Payees()
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

// ---- Transactions / Directives accessors ----

func TestFile_Transactions(t *testing.T) {
	f := makeFile(
		dir("account", "A"),
		tx("T1"),
		dir("account", "B"),
		tx("T2"),
	)
	ts := f.Transactions()
	if len(ts) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(ts))
	}
	if ts[0].Description != "T1" || ts[1].Description != "T2" {
		t.Errorf("wrong transactions: %+v", ts)
	}
}

func TestFile_Directives(t *testing.T) {
	f := makeFile(
		dir("account", "A"),
		tx("T1"),
		dir("account", "B"),
	)
	ds := f.Directives()
	if len(ds) != 2 {
		t.Fatalf("expected 2 directives, got %d", len(ds))
	}
	if ds[0].Argument != "A" || ds[1].Argument != "B" {
		t.Errorf("wrong directives: %+v", ds)
	}
}
