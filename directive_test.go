package ledger

import (
	"testing"
)

func TestDirective_String(t *testing.T) {
	tests := []struct {
		name string
		d    Directive
		want string
	}{
		{
			name: "minimal",
			d:    Directive{Type: "account", Argument: "Expenses:Food"},
			want: "account Expenses:Food\n",
		},
		{
			name: "one line",
			d: Directive{
				Type:     "account",
				Argument: "Expenses:Food",
				Lines:    []string{"note Test Account"},
			},
			want: "account Expenses:Food\n\tnote Test Account\n",
		},
		{
			name: "multiple lines",
			d: Directive{
				Type:     "commodity",
				Argument: "USD",
				Lines:    []string{"format $1,000.00", "alias $"},
			},
			want: "commodity USD\n\tformat $1,000.00\n\talias $\n",
		},
		{
			name: "empty argument",
			d:    Directive{Type: "include", Lines: []string{"other.ledger"}},
			want: "include \n\tother.ledger\n",
		},
		{
			name: "empty type and argument",
			d:    Directive{Lines: []string{"line1", "line2"}},
			want: " \n\tline1\n\tline2\n",
		},
		{
			name: "empty everything",
			d:    Directive{},
			want: " \n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.d.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDirective_Compare(t *testing.T) {
	base := Directive{
		Type:     "account",
		Argument: "Expenses:Food",
		Lines:    []string{"note Test", "alias Food"},
	}

	tests := []struct {
		name string
		a, b Directive
		want bool
	}{
		{
			name: "identical",
			a:    base,
			b:    base,
			want: true,
		},
		{
			name: "different type",
			a:    base,
			b:    Directive{Type: "payee", Argument: "Expenses:Food", Lines: []string{"note Test", "alias Food"}},
			want: false,
		},
		{
			name: "different argument",
			a:    base,
			b:    Directive{Type: "account", Argument: "Assets:Cash", Lines: []string{"note Test", "alias Food"}},
			want: false,
		},
		{
			name: "different lines length",
			a:    base,
			b:    Directive{Type: "account", Argument: "Expenses:Food", Lines: []string{"note Test"}},
			want: false,
		},
		{
			name: "different lines content",
			a:    base,
			b:    Directive{Type: "account", Argument: "Expenses:Food", Lines: []string{"note Test", "alias Groceries"}},
			want: false,
		},
		{
			name: "both empty",
			a:    Directive{},
			b:    Directive{},
			want: true,
		},
		{
			name: "empty vs non-empty type",
			a:    Directive{},
			b:    Directive{Type: "account"},
			want: false,
		},
		{
			name: "nil lines vs empty lines",
			a:    Directive{Type: "x", Argument: "y"},
			b:    Directive{Type: "x", Argument: "y", Lines: []string{}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Compare(tt.b)
			if got != tt.want {
				t.Errorf("Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}
