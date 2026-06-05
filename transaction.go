/*
Copyright 2021 by Milo Christiansen

This software is provided 'as-is', without any express or implied warranty. In
no event will the authors be held liable for any damages arising from the use of
this software.

Permission is granted to anyone to use this software for any purpose, including
commercial applications, and to alter it and redistribute it freely, subject to
the following restrictions:

1. The origin of this software must not be misrepresented; you must not claim
that you wrote the original software. If you use this software in a product, an
acknowledgment in the product documentation would be appreciated but is not
required.

2. Altered source versions must be plainly marked as such, and must not be
misrepresented as being the original software.

3. This notice may not be removed or altered from any source distribution.
*/

/*
Package Ledger contains a parser for Ledger CLI transactions.

This should support the spec more-or-less fully for simple transactions,
but I did not add support for automated transactions or budgeting.

Additionally, I properly implemented String on everything so you can dump
Transactions to a file and read it with Ledger again.

Finally, there are a bunch of functions and methods for dealing with
transactions that should be helpful to anyone trying to use this for
real work.
*/
package ledger

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/milochristiansen/ledger/parse/lex"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type status int

// Status constants for Transaction.Status
const (
	StatusUndefined = status(iota)
	StatusPending
	StatusClear
)

// Transaction is a single transaction from a ledger file.
type Transaction struct {
	Date        time.Time // 2020/10/10
	ClearDate   time.Time // =2020/10/10 (optional)
	Status      status    //   | ! | * (optional)
	Code        string    // ( Stuff ) (optional)
	Description string    // Spent monie on stuf

	Postings []Posting

	Comments []string // ; Stuff...

	Tags    map[string]bool   // ; :tag:tag:tag:
	KVPairs map[string]string // ; Key: Value

	Location lex.Location // The line number where the transaction starts.
}

// Posting is a single line item in a Transaction.
type Posting struct {
	Status    status //   | ! | *  (optional)
	Account   string // Account:Name
	Value     int64  // $20.00 (currently only supporting USD, in thousandths of a cent)
	Null      bool   // True if the Value is implied. Value may or may not contain a valid amount.
	Assert    int64  // = $20.00
	HasAssert bool
	Note      string // ; Stuff
}

// CleanCopy takes a perfect copy of the transaction object, safe for editing without making any changes to the parent.
func (t *Transaction) CleanCopy() *Transaction {
	nt := *t
	nt.Postings = slices.Clone(t.Postings)
	nt.Comments = slices.Clone(t.Comments)
	nt.Tags = maps.Clone(t.Tags)
	nt.KVPairs = maps.Clone(t.KVPairs)
	return &nt
}

// Balance ensures that all postings in the transaction add up to 0 or there is a single null posting.
// Returns false, nil if there is more than one null posting, otherwise returns the ending balances of
// all accounts with postings and true if the transaction balances to 0 or there was a null posting.
func (t *Transaction) Balance() (bool, map[string]int64) {
	bal := int64(0)
	null := -1
	accounts := map[string]int64{}

	for i, p := range t.Postings {
		if p.Null && null != -1 {
			return false, nil // Multiple null postings
		}
		if p.Null {
			null = i
			continue
		}
		bal += p.Value
		accounts[p.Account] += p.Value
	}
	if null != -1 {
		accounts[t.Postings[null].Account] += -bal
		return true, accounts
	}
	return bal == 0, accounts
}

// Canonicalize takes a transaction and sets the value of any null postings that may exist to
// the required value to make it balance. Returns an error if there are multiple null postings or
// if there are no null postings and the transaction does not balance.
func (t *Transaction) Canonicalize() error {
	bal := int64(0)
	null := -1

	for i, p := range t.Postings {
		if p.Null && null != -1 {
			return MultipleNullError{-1, t.Location}
		}
		if p.Null {
			null = i
			continue
		}
		bal += p.Value
	}
	if null != -1 {
		t.Postings[null].Value = -bal
		return nil
	}
	if bal != 0 {
		return BalanceError{-1, t.Location}
	}
	return nil
}

// SumTransactions balances a list of transactions, and returns a map of accounts to their ending values.
func SumTransactions(ts []Transaction) (map[string]int64, error) {
	accounts := map[string]int64{}

	for i, t := range ts {
		ok, ac := t.Balance()
		if !ok {
			return nil, BalanceError{i, t.Location}
		}

		for k, v := range ac {
			accounts[k] += v
		}
	}

	return accounts, nil
}

type sumTree struct {
	children map[string]*sumTree
	value    int64
}

func (st *sumTree) render(name, lvl, pad string, res [][]string) [][]string {
	if len(st.children) == 1 {
		for key, child := range st.children {
			if name == "" {
				return child.render(key, lvl, pad, res)
			}
			return child.render(name+":"+key, lvl, pad, res)
		}
	}

	padding := ""
	if name != "" {
		padding = pad
		res = append(res, []string{lvl + name, FormatValue(st.value)})
	}

	keys := make([]string, 0, len(st.children))
	for key := range st.children {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		res = st.children[key].render(key, lvl+padding, pad, res)
	}
	return res
}

// FormatSums takes a map of accounts to sums and turns it into a list of name/value pairs
// with indentation applied to the names.
func FormatSums(accounts map[string]int64, pad string) [][]string {
	// Generate an accounts tree
	root := &sumTree{children: map[string]*sumTree{}}

	for account, value := range accounts {
		parts := strings.Split(account, ":")

		level := root
		for _, part := range parts {
			if level.children == nil {
				level.children = map[string]*sumTree{}
			}
			if level.children[part] == nil {
				level.children[part] = &sumTree{}
			}
			level.children[part].value += value
			level = level.children[part]
		}
	}

	return root.render("", "", pad, nil)
}

func (t *Transaction) entryType() {}
func (t *Transaction) String() string {
	buf := new(bytes.Buffer)

	buf.WriteString(t.Date.Format("2006/01/02"))
	if !t.ClearDate.IsZero() {
		fmt.Fprintf(buf, "=%v", t.ClearDate.Format("2006/01/02"))
	}

	switch t.Status {
	case StatusClear:
		buf.WriteString(" * ")
	case StatusPending:
		buf.WriteString(" ! ")
	default:
		buf.WriteString("   ")
	}

	if t.Code != "" {
		fmt.Fprintf(buf, "(%v) ", t.Code)
	}

	fmt.Fprintf(buf, "%v\n", t.Description)

	// We don't know if the comments and postings were interleaved in any way,
	// so canonically we will just do the comments and metadata first.
	for _, line := range t.Comments {
		fmt.Fprintf(buf, "\t; %v\n", line)
	}
	if len(t.Tags) != 0 {
		fmt.Fprint(buf, "\t; ")
		for tag := range t.Tags {
			fmt.Fprintf(buf, ":%v", tag)
		}
		fmt.Fprint(buf, ":\n")
	}
	for k, v := range t.KVPairs {
		fmt.Fprintf(buf, "\t; %v: %v\n", k, v)
	}

	for _, p := range t.Postings {
		fmt.Fprintf(buf, "\t%v\n", p.String())
	}

	return buf.String()
}

func (p *Posting) String() string {
	buf := new(bytes.Buffer)

	switch p.Status {
	case StatusClear:
		buf.WriteString("* ")
	case StatusPending:
		buf.WriteString("! ")
	default:
		// This would pad all lines to the same length, but since these clear indicators are not common
		// adding them would just look like a bug (ask me how I know...)
		//buf.WriteString("  ")
	}

	if !p.Null {
		// In order to align on the decimal point instead of the first digit, we need to figure out how much value is
		// before the decimal point so we can reduce the account padding to match.
		value := FormatValue(p.Value)
		prefixlen := strings.Index(value, ".")
		// pad cannot go negative with int64 values: max prefix is ~19 chars
		// for $922337203685477, well under the 62-char pad width.
		pad := 62 - prefixlen
		// We write the account name, pad it out taking into account the length of the value (align at the decimal
		// point), add an extra two spaces so we don't need to write a bunch of logic for pathologically long account
		// names, and then write the value.
		fmt.Fprintf(buf, "%-*s  %s", pad, p.Account, value)

		if p.HasAssert {
			buf.WriteString(" = ")
			buf.WriteString(FormatValue(p.Assert))
		}
	} else {
		if p.HasAssert {
			fmt.Fprintf(buf, "%-62s      = %s", p.Account, FormatValue(p.Assert))
		} else {
			buf.WriteString(p.Account)
		}
	}

	if p.Note != "" {
		fmt.Fprintf(buf, " ; %v", p.Note)
	}

	return buf.String()
}

// ParseValueNumber takes a decimal number string and converts it to an integer
// representing ten-thousandths of the base currency unit (e.g. "123.4567" → 1234567).
// Rounding is done via the round-to-even method.
func ParseValueNumber(v string) (int64, error) {
	neg := false
	if len(v) > 0 && v[0] == '-' {
		neg = true
		v = v[1:]
	}

	var whole, frac4, fifthDigit int64
	fracDigits := 0
	hasRemainder := false
	inFrac := false

	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '.' {
			if inFrac {
				return 0, fmt.Errorf("invalid number: multiple decimal points in %q", v)
			}
			inFrac = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number: unexpected character %q in %q", string(c), v)
		}
		digit := int64(c - '0')
		if inFrac {
			switch {
			case fracDigits < 4:
				frac4 = frac4*10 + digit
			case fracDigits == 4:
				fifthDigit = digit
			default:
				if digit != 0 {
					hasRemainder = true
				}
			}
			fracDigits++
		} else {
			whole = whole*10 + digit
		}
	}

	// Pad or round fractional part to exactly 4 digits.
	switch {
	case fracDigits < 4:
		for fracDigits < 4 {
			frac4 *= 10
			fracDigits++
		}
	case fracDigits >= 5:
		roundUp := fifthDigit > 5 || (fifthDigit == 5 && (hasRemainder || frac4%2 != 0))
		if roundUp {
			frac4++
			if frac4 > 9999 {
				frac4 = 0
				whole++
			}
		}
	}

	result := whole*10000 + frac4
	if neg {
		result = -result
	}
	return result, nil
}

func FormatValue(v int64) string {
	neg, ms, ls1, ls2 := formatHelper(v)
	if neg {
		return fmt.Sprintf("-$%v.%v%v", ms, ls1, ls2)
	}
	return fmt.Sprintf("$%v.%v%v", ms, ls1, ls2)
}

// FormatValueNumber is exactly the same as FormatValue, but it does not add any currency indicators.
func FormatValueNumber(v int64) string {
	neg, ms, ls1, ls2 := formatHelper(v)
	if neg {
		return fmt.Sprintf("-%v.%v%v", ms, ls1, ls2)
	}
	return fmt.Sprintf("%v.%v%v", ms, ls1, ls2)
}

func formatHelper(v int64) (neg bool, ms, ls1, ls2 int64) {
	if v < 0 {
		neg = true
		v = -v
	}
	ms = v / 10000
	ls := v % 10000 / 100
	ls = roundToEven(ls, v%100/10)
	if ls > 99 {
		ls = 0
		ms++
	}

	ls1 = ls / 10
	ls2 = ls % 10

	return
}

// roundToEven rounds ms to the nearest even integer when the next digit is ls.
// ms must be non-negative. ls must be a value between 0 and 9.
func roundToEven(ms, ls int64) int64 {
	if ls > 5 || (ls == 5 && ms%2 != 0) {
		return ms + 1
	}
	return ms
}


// Error types

// BalanceError is returned by functions that validate transactions in some way when the transaction isn't balanced.
type BalanceError struct {
	T int
	L lex.Location
}

func (err BalanceError) Error() string {
	if err.T < 0 {
		return fmt.Sprintf("Transaction (defined on line %v) does not balance.", err.L)
	}
	return fmt.Sprintf("Transaction %v (defined on line %v) does not balance.", err.T, err.L)
}

// MultipleNullError is returned by functions that validate transactions in some way when the transaction has more
// than one null posting.
type MultipleNullError struct {
	T int
	L lex.Location
}

func (err MultipleNullError) Error() string {
	if err.T < 0 {
		return fmt.Sprintf("Transaction (defined on line %v) has multiple null postings.", err.L)
	}
	return fmt.Sprintf("Transaction %v (defined on line %v) has multiple null postings.", err.T, err.L)
}
