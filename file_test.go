package ledger

import (
	"bytes"
	"strings"
	"testing"
)

// ---- Format ----

func TestFile_Format(t *testing.T) {
	f := makeFile

	t.Run("empty_file", func(t *testing.T) {
		var buf bytes.Buffer
		err := f().Format(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected empty output, got %q", buf.String())
		}
	})

	t.Run("transactions_only", func(t *testing.T) {
		var buf bytes.Buffer
		err := f(tx("First"), tx("Second")).Format(&buf)
		if err != nil {
			t.Fatal(err)
		}
		s := buf.String()
		if !strings.Contains(s, "First") || !strings.Contains(s, "Second") {
			t.Errorf("missing transactions: %q", s)
		}
	})

	t.Run("directives_only", func(t *testing.T) {
		var buf bytes.Buffer
		err := f(dir("account", "Expenses:Food"), dir("account", "Assets:Cash")).Format(&buf)
		if err != nil {
			t.Fatal(err)
		}
		s := buf.String()
		if !strings.Contains(s, "Expenses:Food") || !strings.Contains(s, "Assets:Cash") {
			t.Errorf("missing directives: %q", s)
		}
	})

	t.Run("interleaved", func(t *testing.T) {
		var buf bytes.Buffer
		err := f(
			dir("account", "Before"),
			tx("T1"),
			dir("account", "Middle"),
			tx("T2"),
			dir("account", "After"),
		).Format(&buf)
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
