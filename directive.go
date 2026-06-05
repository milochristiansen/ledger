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
	"bytes"

	"github.com/milochristiansen/ledger/parse/lex"
)

// Directive is a simple type to represent a partially parsed, but not validated, command directive.
type Directive struct {
	Type     string       // The keyword that starts the directive.
	Argument string       // Any remaining content that was on the first line of the directive.
	Lines    []string     // Subsequent indented lines. Stored here unparsed.
	Location lex.Location // Line number this directive begins at.
	Parsed   ParsedDirective
}

// ParsedDirective is implemented by types that represent a parsed directive.
// Only types in this package may implement it.
type ParsedDirective interface {
	parsedDirective()
}

func (d *Directive) entryType() {}

func (d *Directive) String() string {
	buf := new(bytes.Buffer)

	buf.WriteString(d.Type)
	buf.WriteRune(' ')
	buf.WriteString(d.Argument)
	buf.WriteRune('\n')

	for _, line := range d.Lines {
		buf.WriteRune('\t')
		buf.WriteString(line)
		buf.WriteRune('\n')
	}

	return buf.String()
}

// Compare two directives to see if they are identical.
func (d *Directive) Compare(d2 Directive) bool {
	ok := d.Type == d2.Type && d.Argument == d2.Argument && len(d.Lines) == len(d2.Lines)
	if !ok {
		return false
	}
	for i := 0; i < len(d.Lines); i++ {
		if d.Lines[i] != d2.Lines[i] {
			return false
		}
	}
	return true
}

// ---- Parsed directive types ----

// Account is a simple type representing an account directive.
type Account struct {
	Name    string   // The name of this account.
	Note    string   // The contents of the note subdirective.
	Aliases []string // One string for each alias subdirective.
	Payees  []string // One string for each payee subdirective.
	Default bool     // True if the default subdirective is present.

	Location lex.Location // Line number where this account starts.
}

func (a Account) parsedDirective() {}

// Payee is a simple type representing a payee directive.
type Payee struct {
	Name    string   // The payee name to substitute if matched
	Aliases []string // One string for each regexp to match with.
	Uuids   []string // One string for each uuid to check.

	Location lex.Location // Line number where this directive starts.
}

func (p Payee) parsedDirective() {}

// Include represents a parsed include directive.
type Include struct {
	Path string // The path from the include directive.
	File *File  // The parsed file, or nil if parsing failed.
	Err  error  // Any error encountered during loading or parsing.
}

func (p Include) parsedDirective() {}

// Comment represents a standalone comment line or block of comment lines.
type Comment struct {
	Lines []string // The comment text, one entry per line (without the leading ;).
}

func (c *Comment) entryType() {}

func (c *Comment) String() string {
	buf := new(bytes.Buffer)
	for _, line := range c.Lines {
		buf.WriteString("; ")
		buf.WriteString(line)
		buf.WriteRune('\n')
	}
	return buf.String()
}
