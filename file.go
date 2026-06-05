package ledger

import (
	"errors"
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

// walk calls fn for each entry in the file, descending into parsed include directives.
func (f *File) walk(fn func(Entry)) {
	for _, e := range f.Entries {
		if d, ok := e.(*Directive); ok && d.Type == "include" {
			if pi, ok := d.Parsed.(*Include); ok && pi.File != nil {
				pi.File.walk(fn)
				continue
			}
		}
		fn(e)
	}
}

// Transactions returns all transaction entries in file order, descending into includes.
func (f *File) Transactions() []Transaction {
	var ts []Transaction
	f.walk(func(e Entry) {
		if t, ok := e.(*Transaction); ok {
			ts = append(ts, *t)
		}
	})
	return ts
}

// Directives returns all directive entries in file order, descending into includes.
func (f *File) Directives() []Directive {
	var ds []Directive
	f.walk(func(e Entry) {
		if d, ok := e.(*Directive); ok {
			ds = append(ds, *d)
		}
	})
	return ds
}

// Format writes out a ledger file from the entries in order. Includes are not resolved.
// Consecutive directives of the same type are grouped without blank-line separators.
func (f *File) Format(w io.Writer) error {
	var prevType string
	for i, e := range f.Entries {
		curType := ""
		if d, ok := e.(*Directive); ok {
			curType = d.Type
		}
		sep := "\n"
		if i > 0 && curType != "" && curType == prevType {
			sep = ""
		}
		fmt.Fprintf(w, "%s%v", sep, e.String())
		prevType = curType
	}
	return nil
}

// Includes processes all include directives in the file. For each, it calls load
// with the path, storing the result as an Include in the directive's Parsed field.
// Already-loaded includes (File != nil) are descended into recursively.
// Errors from individual includes are collected and returned as a joined error.
func (f *File) Includes(load func(path string) (*File, error)) ([]*Include, error) {
	var pis []*Include
	var errs []error
	for _, e := range f.Entries {
		d, ok := e.(*Directive)
		if !ok || d.Type != "include" {
			continue
		}

		if pi, ok := d.Parsed.(*Include); ok {
			pis = append(pis, pi)
			if pi.File != nil {
				sub, err := pi.File.Includes(load)
				if err != nil {
					errs = append(errs, err)
				}
				pis = append(pis, sub...)
			}
			continue
		}

		pi := &Include{Path: d.Argument}
		included, err := load(d.Argument)
		if err != nil {
			pi.Err = err
			errs = append(errs, err)
		} else {
			pi.File = included
			sub, err := included.Includes(load)
			if err != nil {
				errs = append(errs, err)
			}
			pis = append(pis, sub...)
		}
		d.Parsed = pi
		pis = append(pis, pi)
	}
	return pis, errors.Join(errs...)
}

// ErrMalformedAccountName is returned by File.Accounts if an account name is malformed.
type ErrMalformedAccountName struct {
	Name     string
	Location lex.Location
}

func (err ErrMalformedAccountName) Error() string {
	return fmt.Sprintf("Malformed account name (%s) at %s", err.Name, err.Location)
}

// Accounts returns a slice of all account directives, in file order, descending into includes.
// If any account directives fail to parse, Accounts returns an error.
func (f *File) Accounts() ([]Account, error) {
	accts := []Account{}
	var walkErr error
	f.walk(func(e Entry) {
		if walkErr != nil {
			return
		}
		d, ok := e.(*Directive)
		if !ok || d.Type != "account" {
			return
		}

		if parsed, ok := d.Parsed.(Account); ok {
			accts = append(accts, parsed)
			return
		}

		acct := Account{
			Name:     d.Argument,
			Location: d.Location,
		}

		if strings.Contains(acct.Name, "  ") || strings.ContainsAny(acct.Name, ";\t") {
			walkErr = ErrMalformedAccountName{acct.Name, acct.Location}
			return
		}

		for sdiIx, sd := range d.Lines {
			if strings.HasPrefix(sd, "default") {
				acct.Default = true
			} else if alias, ok := strings.CutPrefix(sd, "alias"); ok {
				alias = strings.TrimSpace(alias)
				if strings.Contains(alias, "  ") || strings.ContainsAny(alias, ";\t") {
					walkErr = ErrMalformedAccountName{
						Name:     alias,
						Location: acct.Location.L(acct.Location.Line() + uint64(sdiIx)),
					}
					return
				}
				acct.Aliases = append(acct.Aliases, alias)
			} else if payee, ok := strings.CutPrefix(sd, "payee"); ok {
				payee = strings.TrimSpace(payee)
				acct.Payees = append(acct.Payees, payee)
			} else if note, ok := strings.CutPrefix(sd, "note"); ok {
				note = strings.TrimSpace(note)
				acct.Note = note
			}
		}

		d.Parsed = acct
		accts = append(accts, acct)
	})
	return accts, walkErr
}

// Payees returns a slice of all payee directives, in file order, descending into includes.
func (f *File) Payees() ([]Payee, error) {
	payees := []Payee{}
	f.walk(func(e Entry) {
		d, ok := e.(*Directive)
		if !ok || d.Type != "account" {
			return
		}

		if parsed, ok := d.Parsed.(Payee); ok {
			payees = append(payees, parsed)
			return
		}

		payee := Payee{
			Name:     d.Argument,
			Location: d.Location,
		}

		for _, sd := range d.Lines {
			if alias, ok := strings.CutPrefix(sd, "alias"); ok {
				alias = strings.TrimSpace(alias)
				payee.Aliases = append(payee.Aliases, alias)
			} else if uuid, ok := strings.CutPrefix(sd, "uuid"); ok {
				uuid = strings.TrimSpace(uuid)
				payee.Uuids = append(payee.Uuids, uuid)
			}
		}

		d.Parsed = payee
		payees = append(payees, payee)
	})
	return payees, nil
}
