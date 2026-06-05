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

package ledger

import (
	"fmt"
	"io"
	"strings"

	"github.com/milochristiansen/ledger/parse/lex"
)

// Entry is a single element in a ledger file — either a *Transaction or a *Directive.
type Entry interface {
	String() string
	entryType() // sealed: only Transaction and Directive implement this
}

// File holds a parsed ledger file as an ordered list of entries.
type File struct {
	Entries []Entry // each element is *Transaction or *Directive
}

// Transactions returns all transaction entries in file order.
func (f *File) Transactions() []Transaction {
	var ts []Transaction
	for _, e := range f.Entries {
		if t, ok := e.(*Transaction); ok {
			ts = append(ts, *t)
		}
	}
	return ts
}

// Directives returns all directive entries in file order.
func (f *File) Directives() []Directive {
	var ds []Directive
	for _, e := range f.Entries {
		if d, ok := e.(*Directive); ok {
			ds = append(ds, *d)
		}
	}
	return ds
}

// Format writes out a ledger file from the entries in order.
func (f *File) Format(w io.Writer) error {
	for _, e := range f.Entries {
		fmt.Fprintf(w, "\n%v", e.String())
	}
	return nil
}

// ErrMalformedAccountName is returned by File.Accounts if an account name is malformed.
type ErrMalformedAccountName struct {
	Name     string
	Location lex.Location
}

func (err ErrMalformedAccountName) Error() string {
	return fmt.Sprintf("Malformed account name (%s) at %s", err.Name, err.Location)
}

// Accounts returns a slice of all account directives, in file order.
// If any account directives fail to parse, Accounts returns an error.
func (f *File) Accounts() ([]Account, error) {
	accts := []Account{}
	for _, e := range f.Entries {
		d, ok := e.(*Directive)
		if !ok || d.Type != "account" {
			continue
		}

		acct := Account{
			Name:     d.Argument,
			Location: d.Location,
		}

		// filter out some things that cause funny behavior
		if strings.Contains(acct.Name, "  ") || strings.ContainsAny(acct.Name, ";\t") {
			return nil, ErrMalformedAccountName{acct.Name, acct.Location}
		}

		for sdiIx, sd := range d.Lines {
			if strings.HasPrefix(sd, "default") {
				acct.Default = true
			} else if strings.HasPrefix(sd, "alias") {
				alias := strings.TrimSpace(sd[len("alias"):])
				if strings.Contains(alias, "  ") || strings.ContainsAny(alias, ";\t") {
					return nil, ErrMalformedAccountName{
						Name:     alias,
						Location: acct.Location.L(acct.Location.Line() + uint64(sdiIx)),
					}
				}
				acct.Aliases = append(acct.Aliases, alias)
			} else if strings.HasPrefix(sd, "payee") {
				payee := strings.TrimSpace(sd[len("payee"):])
				acct.Payees = append(acct.Payees, payee)
			} else if strings.HasPrefix(sd, "note") {
				note := strings.TrimSpace(sd[len("note"):])
				acct.Note = note
			}
		}

		accts = append(accts, acct)
	}
	return accts, nil
}

// Payees returns a slice of all payee directives, in file order.
func (f *File) Payees() ([]Payee, error) {
	payees := []Payee{}
	for _, e := range f.Entries {
		d, ok := e.(*Directive)
		if !ok || d.Type != "account" {
			continue
		}

		payee := Payee{
			Name:     d.Argument,
			Location: d.Location,
		}

		for _, sd := range d.Lines {
			if strings.HasPrefix(sd, "alias") {
				alias := strings.TrimSpace(sd[len("alias"):])
				payee.Aliases = append(payee.Aliases, alias)
			} else if strings.HasPrefix(sd, "uuid") {
				uuid := strings.TrimSpace(sd[len("uuid"):])
				payee.Uuids = append(payee.Uuids, uuid)
			}
		}

		payees = append(payees, payee)
	}
	return payees, nil
}

// Account is a simple type representing an account directive.
type Account struct {
	Name    string   // The name of this account.
	Note    string   // The contents of the note subdirective.
	Aliases []string // One string for each alias subdirective.
	Payees  []string // One string for each payee subdirective.
	Default bool     // True if the default subdirective is present.

	Location lex.Location // Line number where this account starts.
}

// Payee is a simple type representing a payee directive.
type Payee struct {
	Name    string   // The payee name to substitute if matched
	Aliases []string // One string for each regexp to match with.
	Uuids   []string // One string for each uuid to check.

	Location lex.Location // Line number where this directive starts.
}
