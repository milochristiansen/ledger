package main

import (
	"testing"
)

// Test that flagValue parses correctly.
func TestFlagValue_Set(t *testing.T) {
	var f flagValue

	if err := f.Set("description=New Value"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f))
	}
	if f[0].key != "description" || f[0].value != "New Value" {
		t.Errorf("got %q=%q, want description=New Value", f[0].key, f[0].value)
	}

	if err := f.Set("account:0=Expenses:Food"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(f))
	}
	if f[1].key != "account:0" || f[1].value != "Expenses:Food" {
		t.Errorf("got %q=%q, want account:0=Expenses:Food", f[1].key, f[1].value)
	}
}

func TestFlagValue_Set_MissingEquals(t *testing.T) {
	var f flagValue
	if err := f.Set("noequals"); err == nil {
		t.Error("expected error for missing =")
	}
}

func TestFlagValue_Set_EmptyValue(t *testing.T) {
	var f flagValue
	if err := f.Set("amount:1="); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f[0].key != "amount:1" || f[0].value != "" {
		t.Errorf("got %q=%q, want amount:1=", f[0].key, f[0].value)
	}
}

func TestFlagValue_String(t *testing.T) {
	var f flagValue
	if s := f.String(); s != "[]" {
		t.Errorf("empty String() = %q, want []", s)
	}
	f.Set("a=b")
	if s := f.String(); s == "[]" {
		t.Error("non-empty String() should not be []")
	}
}

func TestFlagDefaults(t *testing.T) {
	if *refFlag != "" {
		t.Error("refFlag default should be empty")
	}
	if *fileFlag != "" {
		t.Error("fileFlag default should be empty")
	}
	if len(sets) != 0 {
		t.Error("sets default should be empty")
	}
}
