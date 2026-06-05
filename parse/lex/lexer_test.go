package lex

import (
	"strings"
	"testing"
)

// ---- NewCharReader ----

func TestNewCharReader(t *testing.T) {
	cr := NewCharReader("ab", 100)
	if cr.EOF {
		t.Error("expected not EOF")
	}
	if cr.C != 'a' {
		t.Errorf("C = %q, want 'a'", cr.C)
	}
	if cr.NC != 'b' {
		t.Errorf("NC = %q, want 'b'", cr.NC)
	}
	if cr.L.Line() != 100 {
		t.Errorf("line = %d, want 100", cr.L.Line())
	}
}

func TestNewRawCharReader(t *testing.T) {
	cr := NewRawCharReader(strings.NewReader("xy"), 5)
	if cr.C != 'x' || cr.NC != 'y' {
		t.Errorf("C=%q NC=%q, want 'x' 'y'", cr.C, cr.NC)
	}
}

// ---- Match / NMatch ----

func TestMatch(t *testing.T) {
	cr := NewCharReader("abc", 1)
	if !cr.Match("ab") {
		t.Error("expected match on 'a'")
	}
	cr.Next()
	if !cr.Match("abc") {
		t.Error("expected match on 'b'")
	}
	cr.Next()
	if !cr.Match("c") {
		t.Error("expected match on 'c'")
	}
}

func TestMatch_NoMatch(t *testing.T) {
	cr := NewCharReader("x", 1)
	if cr.Match("ab") {
		t.Error("expected no match")
	}
}

func TestMatch_EOF(t *testing.T) {
	cr := NewCharReader("a", 1)
	cr.Next() // move past 'a'
	cr.Next() // EOF
	if cr.Match("a") {
		t.Error("expected false at EOF")
	}
}

func TestNMatch(t *testing.T) {
	cr := NewCharReader("abc", 1)
	if !cr.NMatch("b") {
		t.Error("expected NC='b' to match 'b'")
	}
	if cr.NMatch("a") {
		t.Error("expected NC='b' not to match 'a'")
	}
	// Advance to end, verify NEOF
	cr.Next()
	cr.Next()
	cr.Next() // now NEOF should be true
	if cr.NMatch("x") {
		t.Error("expected false at NEOF")
	}
}

// ---- MatchAlpha / MatchNumeric ----

func TestMatchAlpha(t *testing.T) {
	tests := []struct {
		input rune
		want  bool
	}{
		{'a', true},
		{'Z', true},
		{'_', true},
		{'1', false},
		{' ', false},
		{'\n', false},
	}
	for _, tt := range tests {
		cr := NewRawCharReader(strings.NewReader(string(tt.input)), 1)
		if cr.MatchAlpha() != tt.want {
			t.Errorf("MatchAlpha(%q) = %v, want %v", tt.input, !tt.want, tt.want)
		}
	}
}

func TestMatchAlpha_EOF(t *testing.T) {
	cr := NewCharReader("a", 1)
	cr.Next()
	cr.Next()
	if cr.MatchAlpha() {
		t.Error("expected false at EOF")
	}
}

func TestMatchNumeric(t *testing.T) {
	tests := []struct {
		input rune
		want  bool
	}{
		{'0', true},
		{'5', true},
		{'9', true},
		{'a', false},
		{'/', false},
		{':', false},
	}
	for _, tt := range tests {
		cr := NewRawCharReader(strings.NewReader(string(tt.input)), 1)
		if cr.MatchNumeric() != tt.want {
			t.Errorf("MatchNumeric(%q) = %v, want %v", tt.input, !tt.want, tt.want)
		}
	}
}

func TestMatchNumeric_EOF(t *testing.T) {
	cr := NewCharReader("1", 1)
	cr.Next()
	cr.Next()
	if cr.MatchNumeric() {
		t.Error("expected false at EOF")
	}
}

// ---- Next ----

func TestNext_Basic(t *testing.T) {
	cr := NewCharReader("abc", 5)
	// Initial: C='a', NC='b'. After priming, col reflects consumed chars.
	if cr.L.Line() != 5 {
		t.Errorf("initial line = %d, want 5", cr.L.Line())
	}

	cr.Next()
	// C='b', NC='c'
	if cr.C != 'b' {
		t.Errorf("C = %q, want 'b'", cr.C)
	}
	if cr.NC != 'c' {
		t.Errorf("NC = %q, want 'c'", cr.NC)
	}
}
func TestNext_EOF(t *testing.T) {
	cr := NewCharReader("ab", 1)
	if cr.EOF {
		t.Error("not EOF yet")
	}
	cr.Next() // C='b'
	if cr.EOF {
		t.Error("still not EOF")
	}
	cr.Next() // now at EOF (NC was already NEOF after reading 'b')
	if !cr.EOF {
		t.Error("expected EOF")
	}
	cr.Next() // no-op at EOF
	if !cr.EOF {
		t.Error("should stay at EOF")
	}
}

func TestNext_Newline(t *testing.T) {
	cr := NewCharReader("a\nb", 10)
	// After construction: C='a', NC='\n', NL already adjusted for the newline.
	cr.Next() // C becomes '\n'
	if cr.L.Line() != 11 {
		t.Errorf("C line after newline = %d, want 11", cr.L.Line())
	}
	if cr.L.Column() != 0 {
		t.Errorf("C col after newline = %d, want 0", cr.L.Column())
	}
}

func TestNext_CRStripping(t *testing.T) {
	cr := NewRawCharReader(strings.NewReader("a\rb"), 1)
	// C='a', NC='b' (CR stripped)
	if cr.NC != 'b' {
		t.Errorf("NC = %q, want 'b' (CR stripped)", cr.NC)
	}
}

// ---- Eat / EatUntil ----

func TestEat(t *testing.T) {
	cr := NewCharReader("   abc", 1)
	cr.Eat(" \t")
	if cr.C != 'a' {
		t.Errorf("C = %q, want 'a'", cr.C)
	}
}

func TestEat_Nothing(t *testing.T) {
	cr := NewCharReader("abc", 1)
	cr.Eat(" \t")
	if cr.C != 'a' {
		t.Errorf("C changed: %q", cr.C)
	}
}

func TestEat_EOF(t *testing.T) {
	cr := NewCharReader("   ", 1)
	cr.Eat(" \t")
	if !cr.EOF {
		t.Error("expected EOF after eating all input")
	}
}

func TestEatUntil(t *testing.T) {
	cr := NewCharReader("abc\n", 1)
	cr.EatUntil("\n")
	if cr.C != '\n' {
		t.Errorf("C = %q, want newline", cr.C)
	}
	// Should not have consumed the newline
	cr.EatUntil("x") // nothing matches, runs to EOF
	if !cr.EOF {
		t.Error("expected EOF")
	}
}

// ---- ReadMatch ----

func TestReadMatch(t *testing.T) {
	cr := NewCharReader("123abc", 1)
	buf := cr.ReadMatch("0123456789", nil)
	if string(buf) != "123" {
		t.Errorf("buf = %q, want '123'", string(buf))
	}
	if cr.C != 'a' {
		t.Errorf("C = %q, want 'a'", cr.C)
	}
}

func TestReadMatch_EOF(t *testing.T) {
	cr := NewCharReader("999", 1)
	buf := cr.ReadMatch("0123456789", nil)
	if string(buf) != "999" {
		t.Errorf("buf = %q, want '999'", string(buf))
	}
	if !cr.EOF {
		t.Error("expected EOF")
	}
}

func TestReadMatch_None(t *testing.T) {
	cr := NewCharReader("abc", 1)
	buf := cr.ReadMatch("0123456789", nil)
	if len(buf) != 0 {
		t.Errorf("expected empty buf, got %q", string(buf))
	}
}

// ---- ReadMatchLimit ----

func TestReadMatchLimit(t *testing.T) {
	cr := NewCharReader("123456", 1)
	limited, buf := cr.ReadMatchLimit("0123456789", nil, 3)
	if !limited {
		t.Error("expected limited=true")
	}
	if string(buf) != "123" {
		t.Errorf("buf = %q, want '123'", string(buf))
	}
}

func TestReadMatchLimit_NotLimited(t *testing.T) {
	cr := NewCharReader("12abc", 1)
	limited, buf := cr.ReadMatchLimit("0123456789", nil, 5)
	if limited {
		t.Error("expected limited=false")
	}
	if string(buf) != "12" {
		t.Errorf("buf = %q, want '12'", string(buf))
	}
}

func TestReadMatchLimit_EOF(t *testing.T) {
	cr := NewCharReader("12", 1)
	limited, buf := cr.ReadMatchLimit("0123456789", nil, 5)
	if limited {
		t.Error("expected limited=false (stopped by EOF)")
	}
	if string(buf) != "12" {
		t.Errorf("buf = %q, want '12'", string(buf))
	}
}

// ---- ReadUntil ----

func TestReadUntil(t *testing.T) {
	cr := NewCharReader("hello world\n", 1)
	buf := cr.ReadUntil(" \n", nil)
	if string(buf) != "hello" {
		t.Errorf("buf = %q, want 'hello'", string(buf))
	}
	if cr.C != ' ' {
		t.Errorf("C = %q, want space", cr.C)
	}
}

func TestReadUntil_EOF(t *testing.T) {
	cr := NewCharReader("hello", 1)
	buf := cr.ReadUntil("\n", nil)
	if string(buf) != "hello" {
		t.Errorf("buf = %q, want 'hello'", string(buf))
	}
	if !cr.EOF {
		t.Error("expected EOF")
	}
}

// ---- Location ----

func TestLocation_LineColumn(t *testing.T) {
	var l Location
	l = l.L(42).C(5)
	if l.Line() != 42 {
		t.Errorf("Line() = %d, want 42", l.Line())
	}
	if l.Column() != 5 {
		t.Errorf("Column() = %d, want 5", l.Column())
	}
}

func TestLocation_String(t *testing.T) {
	var l Location
	l = l.L(100).C(33)
	if s := l.String(); s != "100:33" {
		t.Errorf("String() = %q, want %q", s, "100:33")
	}
}

func TestLocation_L_Overflow(t *testing.T) {
	var l Location
	// Bits beyond 48 should cause a reset to 0
	l = l.L(0x0001_000000000000) // bit 48 set
	if l.Line() != 0 {
		t.Errorf("expected 0 on overflow, got %d", l.Line())
	}
}

func TestLocation_C_Boundary(t *testing.T) {
	var l Location
	l = l.C(65535) // max uint16
	if l.Column() != 65535 {
		t.Errorf("Column() = %d, want 65535", l.Column())
	}
}

func TestLocation_LPlus(t *testing.T) {
	var l Location
	l = l.L(10)
	l = l.LPlus()
	if l.Line() != 11 {
		t.Errorf("LPlus line = %d, want 11", l.Line())
	}
}

func TestLocation_CPlus(t *testing.T) {
	var l Location
	l = l.C(3)
	l = l.CPlus()
	if l.Column() != 4 {
		t.Errorf("CPlus col = %d, want 4", l.Column())
	}
}

func TestLocation_Composability(t *testing.T) {
	// L and C are independent setters
	var l Location
	l = l.L(15).C(7)
	if l.Line() != 15 || l.Column() != 7 {
		t.Errorf("got %v:%v, want 15:7", l.Line(), l.Column())
	}
	// Overwriting one doesn't affect the other
	l = l.L(99)
	if l.Line() != 99 || l.Column() != 7 {
		t.Errorf("L overwrite: got %v:%v, want 99:7", l.Line(), l.Column())
	}
	l = l.C(3)
	if l.Line() != 99 || l.Column() != 3 {
		t.Errorf("C overwrite: got %v:%v, want 99:3", l.Line(), l.Column())
	}
}
