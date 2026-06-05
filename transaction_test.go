package ledger

import (
	"regexp"
	"testing"
	"time"
)

// ---- CleanCopy ----

func TestTransaction_CleanCopy(t *testing.T) {
	orig := &Transaction{
		Date:        time.Date(2021, 9, 25, 0, 0, 0, 0, time.UTC),
		Description: "Test",
		Postings:    []Posting{{Account: "Expenses:Foo", Value: 1000}},
		Comments:    []string{"note"},
		Tags:        map[string]bool{"tag": true},
		KVPairs:     map[string]string{"key": "val"},
	}
	cp := orig.CleanCopy()

	if cp.Description != orig.Description {
		t.Error("description mismatch")
	}
	if len(cp.Postings) != 1 || cp.Postings[0].Account != "Expenses:Foo" {
		t.Error("postings mismatch")
	}
	if len(cp.Comments) != 1 || cp.Comments[0] != "note" {
		t.Error("comments mismatch")
	}
	if !cp.Tags["tag"] {
		t.Error("tags mismatch")
	}
	if cp.KVPairs["key"] != "val" {
		t.Error("kvpairs mismatch")
	}

	// Mutate copy, verify original untouched
	cp.Postings[0].Account = "Changed"
	cp.Comments[0] = "changed"
	cp.Tags["new"] = true
	cp.KVPairs["new"] = "changed"

	if orig.Postings[0].Account != "Expenses:Foo" {
		t.Error("original postings mutated")
	}
	if orig.Comments[0] != "note" {
		t.Error("original comments mutated")
	}
	if orig.Tags["new"] {
		t.Error("original tags mutated")
	}
	if orig.KVPairs["new"] != "" {
		t.Error("original kvpairs mutated")
	}
}

// ---- Balance ----

func TestTransaction_Balance(t *testing.T) {
	t.Run("balanced", func(t *testing.T) {
		tr := &Transaction{Postings: []Posting{
			{Account: "A", Value: 1000},
			{Account: "B", Value: -1000},
		}}
		ok, ac := tr.Balance()
		if !ok {
			t.Error("expected balanced")
		}
		if ac["A"] != 1000 || ac["B"] != -1000 {
			t.Errorf("wrong balances: %v", ac)
		}
	})

	t.Run("unbalanced", func(t *testing.T) {
		tr := &Transaction{Postings: []Posting{
			{Account: "A", Value: 1000},
			{Account: "B", Value: -500},
		}}
		ok, _ := tr.Balance()
		if ok {
			t.Error("expected unbalanced")
		}
	})

	t.Run("single_null_fills_balance", func(t *testing.T) {
		tr := &Transaction{Postings: []Posting{
			{Account: "A", Value: 1000},
			{Account: "B", Null: true},
		}}
		ok, ac := tr.Balance()
		if !ok {
			t.Error("expected balanced via null")
		}
		if ac["B"] != -1000 {
			t.Errorf("null posting not filled: %v", ac)
		}
	})

	t.Run("multiple_null_returns_false", func(t *testing.T) {
		tr := &Transaction{Postings: []Posting{
			{Account: "A", Value: 1000},
			{Account: "B", Null: true},
			{Account: "C", Null: true},
		}}
		ok, ac := tr.Balance()
		if ok {
			t.Error("expected false for multiple nulls")
		}
		if ac != nil {
			t.Error("expected nil accounts for multiple nulls")
		}
	})

	t.Run("no_postings", func(t *testing.T) {
		tr := &Transaction{}
		ok, ac := tr.Balance()
		if !ok {
			t.Error("empty transaction should balance")
		}
		if len(ac) != 0 {
			t.Error("expected empty accounts")
		}
	})

	t.Run("null_only", func(t *testing.T) {
		tr := &Transaction{Postings: []Posting{
			{Account: "A", Null: true},
		}}
		ok, ac := tr.Balance()
		if !ok {
			t.Error("single null should balance")
		}
		if ac["A"] != 0 {
			t.Errorf("null-only posting should fill to 0, got %v", ac["A"])
		}
	})
}

// ---- Canonicalize ----

func TestTransaction_Canonicalize(t *testing.T) {
	t.Run("already_balanced", func(t *testing.T) {
		tr := &Transaction{
			Location: 5,
			Postings: []Posting{
				{Account: "A", Value: 1000},
				{Account: "B", Value: -1000},
			},
		}
		err := tr.Canonicalize()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("fills_null_posting", func(t *testing.T) {
		tr := &Transaction{
			Postings: []Posting{
				{Account: "A", Value: 2000},
				{Account: "B", Null: true},
			},
		}
		err := tr.Canonicalize()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tr.Postings[1].Value != -2000 {
			t.Errorf("null posting not filled: %v", tr.Postings[1].Value)
		}
	})

	t.Run("unbalanced_no_null_returns_error", func(t *testing.T) {
		tr := &Transaction{
			Location: 10,
			Postings: []Posting{
				{Account: "A", Value: 1000},
				{Account: "B", Value: -500},
			},
		}
		err := tr.Canonicalize()
		if err == nil {
			t.Error("expected BalanceError")
		}
		if _, ok := err.(BalanceError); !ok {
			t.Errorf("expected BalanceError, got %T", err)
		}
	})

	t.Run("multiple_null_returns_error", func(t *testing.T) {
		tr := &Transaction{
			Location: 10,
			Postings: []Posting{
				{Account: "A", Value: 1000},
				{Account: "B", Null: true},
				{Account: "C", Null: true},
			},
		}
		err := tr.Canonicalize()
		if err == nil {
			t.Error("expected MultipleNullError")
		}
		if _, ok := err.(MultipleNullError); !ok {
			t.Errorf("expected MultipleNullError, got %T", err)
		}
	})
}

// ---- SumTransactions ----

func TestSumTransactions(t *testing.T) {
	t.Run("sums_correctly", func(t *testing.T) {
		ts := []Transaction{
			{Postings: []Posting{
				{Account: "A", Value: 100},
				{Account: "B", Value: -100},
			}},
			{Postings: []Posting{
				{Account: "A", Value: 200},
				{Account: "C", Value: -200},
			}},
		}
		ac, err := SumTransactions(ts)
		if err != nil {
			t.Fatal(err)
		}
		if ac["A"] != 300 || ac["B"] != -100 || ac["C"] != -200 {
			t.Errorf("wrong sums: %v", ac)
		}
	})

	t.Run("unbalanced_transaction_errors", func(t *testing.T) {
		ts := []Transaction{
			{
				Location: 42,
				Postings: []Posting{
					{Account: "A", Value: 100},
					{Account: "B", Value: -50},
				},
			},
		}
		_, err := SumTransactions(ts)
		if err == nil {
			t.Error("expected BalanceError")
		}
	})

	t.Run("empty_list", func(t *testing.T) {
		ac, err := SumTransactions(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(ac) != 0 {
			t.Error("expected empty map")
		}
	})
}

// ---- FormatSums / sumTree ----

func TestFormatSums(t *testing.T) {
	accounts := map[string]int64{
		"Expenses:Food": 100,
		"Expenses:Rent": 200,
		"Assets:Cash":   -300,
		"Assets:Bank":   50,
		"Liabilities:CC": -50,
	}
	result := FormatSums(accounts, "  ")

	if len(result) == 0 {
		t.Fatal("empty result")
	}

	seen := map[string]bool{}
	for _, row := range result {
		seen[row[0]] = true
	}

	if !seen["Assets"] {
		t.Error("missing Assets row")
	}
	if !seen["Expenses"] {
		t.Error("missing Expenses row")
	}
	if !seen["Liabilities:CC"] {
		t.Error("missing Liabilities:CC row")
	}
}

func TestFormatSums_SingleChildFlattens(t *testing.T) {
	// Single-child chains flatten names: "Parent:Child" without intermediate rows.
	accounts := map[string]int64{
		"Expenses:Only:Sub": 42,
	}
	result := FormatSums(accounts, "  ")
	found := false
	for _, row := range result {
		if row[0] == "Expenses:Only:Sub" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected flattened path 'Expenses:Only:Sub', got %v", result)
	}
}

func TestFormatSums_Empty(t *testing.T) {
	result := FormatSums(map[string]int64{}, "  ")
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}


// ---- Transaction.String ----

func TestTransaction_String(t *testing.T) {
	tr := &Transaction{
		Date:        time.Date(2021, 9, 25, 0, 0, 0, 0, time.UTC),
		Status:      StatusClear,
		Description: "Test Desc",
		Postings: []Posting{
			{Account: "Expenses:Foo", Value: 200000},
			{Account: "Assets:Bar", Value: -200000},
		},
	}
	s := tr.String()
	if s == "" {
		t.Error("empty string output")
	}
	if !regexp.MustCompile(`2021/09/25`).MatchString(s) {
		t.Error("missing date")
	}
	if !regexp.MustCompile(`\*`).MatchString(s) {
		t.Error("missing clear indicator")
	}
	if !regexp.MustCompile(`Test Desc`).MatchString(s) {
		t.Error("missing description")
	}
}

func TestTransaction_String_WithClearDate(t *testing.T) {
	tr := &Transaction{
		Date:        time.Date(2021, 9, 25, 0, 0, 0, 0, time.UTC),
		ClearDate:   time.Date(2021, 9, 28, 0, 0, 0, 0, time.UTC),
		Description: "Test",
	}
	s := tr.String()
	if !regexp.MustCompile(`2021/09/25=2021/09/28`).MatchString(s) {
		t.Errorf("missing clear date: %q", s)
	}
}

func TestTransaction_String_WithCode(t *testing.T) {
	tr := &Transaction{
		Date:        time.Date(2021, 9, 25, 0, 0, 0, 0, time.UTC),
		Code:        "PAY",
		Description: "Test",
	}
	s := tr.String()
	if !regexp.MustCompile(`\(PAY\)`).MatchString(s) {
		t.Errorf("missing code: %q", s)
	}
}

func TestTransaction_String_WithMetadata(t *testing.T) {
	tr := &Transaction{
		Date:        time.Date(2021, 9, 25, 0, 0, 0, 0, time.UTC),
		Description: "Test",
		Comments:    []string{"a comment"},
		Tags:        map[string]bool{"ATag": true},
		KVPairs:     map[string]string{"Key": "Val"},
	}
	s := tr.String()
	if !regexp.MustCompile(`; a comment`).MatchString(s) {
		t.Error("missing comment")
	}
	if !regexp.MustCompile(`:ATag:`).MatchString(s) {
		t.Error("missing tag")
	}
	if !regexp.MustCompile(`Key: Val`).MatchString(s) {
		t.Error("missing kv pair")
	}
}

func TestTransaction_String_PendingStatus(t *testing.T) {
	tr := &Transaction{
		Date:        time.Date(2021, 9, 25, 0, 0, 0, 0, time.UTC),
		Status:      StatusPending,
		Description: "Test",
	}
	s := tr.String()
	if !regexp.MustCompile(`!`).MatchString(s) {
		t.Errorf("missing pending indicator: %q", s)
	}
}

// ---- Posting.String ----

func TestPosting_String(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		p := Posting{Account: "Expenses:Foo", Value: 200000}
		s := p.String()
		if !regexp.MustCompile(`Expenses:Foo`).MatchString(s) {
			t.Error("missing account")
		}
		if !regexp.MustCompile(`\$20\.00`).MatchString(s) {
			t.Errorf("missing value: %q", s)
		}
	})

	t.Run("clear_status", func(t *testing.T) {
		p := Posting{Account: "A", Value: 100, Status: StatusClear}
		s := p.String()
		if !regexp.MustCompile(`^\* `).MatchString(s) {
			t.Errorf("missing clear indicator: %q", s)
		}
	})

	t.Run("pending_status", func(t *testing.T) {
		p := Posting{Account: "A", Value: 100, Status: StatusPending}
		s := p.String()
		if !regexp.MustCompile(`^! `).MatchString(s) {
			t.Errorf("missing pending indicator: %q", s)
		}
	})

	t.Run("null_no_value", func(t *testing.T) {
		p := Posting{Account: "Liabilities:CC", Null: true}
		s := p.String()
		if regexp.MustCompile(`\$`).MatchString(s) {
			t.Errorf("null posting should not show value: %q", s)
		}
		if !regexp.MustCompile(`Liabilities:CC`).MatchString(s) {
			t.Error("missing account")
		}
	})

	t.Run("null_with_assert", func(t *testing.T) {
		p := Posting{Account: "A", Null: true, HasAssert: true, Assert: 52500}
		s := p.String()
		if !regexp.MustCompile(`= \$5\.25`).MatchString(s) {
			t.Errorf("missing assertion on null: %q", s)
		}
	})

	t.Run("with_note", func(t *testing.T) {
		p := Posting{Account: "A", Value: 100, Note: "hello"}
		s := p.String()
		if !regexp.MustCompile(`; hello`).MatchString(s) {
			t.Errorf("missing note: %q", s)
		}
	})

	t.Run("with_assert", func(t *testing.T) {
		p := Posting{Account: "A", Value: 1000, HasAssert: true, Assert: 1000}
		s := p.String()
		if !regexp.MustCompile(`= \$0\.10`).MatchString(s) {
			t.Errorf("missing assertion: %q", s)
		}
	})

	t.Run("negative_value", func(t *testing.T) {
		p := Posting{Account: "A", Value: -5000}
		s := p.String()
		if !regexp.MustCompile(`-\$0\.50`).MatchString(s) {
			t.Errorf("expected -$0.50: %q", s)
		}
	})
}

// ---- ParseValueNumber ----

func TestParseValueNumber(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"0", 0, false},
		{"0.00", 0, false},
		{"123", 1230000, false},
		{"123.45", 1234500, false},
		{"123.4567", 1234567, false},
		{"-123.45", -1234500, false},
		{".45", 4500, false},
		{"123.", 1230000, false},

		{"123456789012345.6789", 1234567890123456789, false},

		{"123.45678", 1234568, false},
		{"123.45672", 1234567, false},
		{"123.45675", 1234568, false},
		{"123.45625", 1234562, false},
		{"123.456250", 1234562, false},
		{"123.456251", 1234563, false},

		{"0.99995", 10000, false},
		{"0.999951", 10000, false},
		{"-0.99995", -10000, false},

		{"42.00000", 420000, false},
		{"42.000001", 420000, false},
		{"42.000005", 420000, false},

		{"1.2.3", 0, true},
		{"abc", 0, true},
		{"12a", 0, true},
		{"12.a", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseValueNumber(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseValueNumber(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseValueNumber(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseValueNumber(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// ---- FormatValue / FormatValueNumber / formatHelper / roundToEven ----

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "$0.00"},
		{100, "$0.01"},
		{200000, "$20.00"},
		{1234567, "$123.46"},
		{99999999900, "$9999999.99"},
		{-200000, "-$20.00"},
		{-1234500, "-$123.45"},
		{-5000, "-$0.50"},
	}

	for _, tt := range tests {
		got := FormatValue(tt.input)
		if got != tt.want {
			t.Errorf("FormatValue(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatValueNumber(t *testing.T) {
	if got := FormatValueNumber(200000); got != "20.00" {
		t.Errorf("FormatValueNumber(200000) = %q, want %q", got, "20.00")
	}
	if got := FormatValueNumber(-5000); got != "-0.50" {
		t.Errorf("FormatValueNumber(-5000) = %q, want %q", got, "-0.50")
	}
}

func TestFormatHelper(t *testing.T) {
	tests := []struct {
		input   int64
		wantNeg bool
		wantMs  int64
		wantLs1 int64
		wantLs2 int64
	}{
		{0, false, 0, 0, 0},
		{10000, false, 1, 0, 0},
		{200000, false, 20, 0, 0},
		{1234500, false, 123, 4, 5},
		{-1234500, true, 123, 4, 5},
		{1234560, false, 123, 4, 6},
		{50, false, 0, 0, 0},       // $0.0050 → even tie → no round
		{150, false, 0, 0, 2},      // $0.0150 → odd tie → round
		{999950, false, 100, 0, 0}, // $99.9950 → carry
	}

	for _, tt := range tests {
		neg, ms, ls1, ls2 := formatHelper(tt.input)
		if neg != tt.wantNeg || ms != tt.wantMs || ls1 != tt.wantLs1 || ls2 != tt.wantLs2 {
			t.Errorf("formatHelper(%d) = (%v, %d, %d, %d), want (%v, %d, %d, %d)",
				tt.input, neg, ms, ls1, ls2, tt.wantNeg, tt.wantMs, tt.wantLs1, tt.wantLs2)
		}
	}
}

func TestRoundToEven(t *testing.T) {
	tests := []struct {
		ms, ls int64
		want   int64
	}{
		{0, 0, 0},
		{0, 4, 0},
		{0, 5, 0},  // 0 is even, tie → no round
		{0, 6, 1},  // 6>5 → round up
		{1, 5, 2},  // 1 is odd, tie → round up
		{2, 5, 2},  // 2 is even, tie → no round
		{9, 5, 10}, // 9 is odd, tie → round up
		{5, 7, 6},  // 7>5 → round up
		{3, 4, 3},  // 4<5 → no round
	}

	for _, tt := range tests {
		got := roundToEven(tt.ms, tt.ls)
		if got != tt.want {
			t.Errorf("roundToEven(%d, %d) = %d, want %d", tt.ms, tt.ls, got, tt.want)
		}
	}
}


// ---- Error types ----

func TestBalanceError_Error(t *testing.T) {
	err := BalanceError{T: 5, L: 100}
	want := "Transaction 5 (defined on line 100:0) does not balance."
	if s := err.Error(); s != want {
		t.Errorf("Error() = %q, want %q", s, want)
	}

	err = BalanceError{T: -1, L: 200}
	want = "Transaction (defined on line 200:0) does not balance."
	if s := err.Error(); s != want {
		t.Errorf("Error() = %q, want %q", s, want)
	}
}

func TestMultipleNullError_Error(t *testing.T) {
	err := MultipleNullError{T: 3, L: 50}
	want := "Transaction 3 (defined on line 50:0) has multiple null postings."
	if s := err.Error(); s != want {
		t.Errorf("Error() = %q, want %q", s, want)
	}

	err = MultipleNullError{T: -1, L: 75}
	want = "Transaction (defined on line 75:0) has multiple null postings."
	if s := err.Error(); s != want {
		t.Errorf("Error() = %q, want %q", s, want)
	}
}
