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

// This is a greatly simplified and stripped down version of the core Lexer code that many of my parsers used for years.
package lex

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// CharReader is a simple way to read from a string character by character, with line info and lookahead.
type CharReader struct {
	source io.RuneReader

	// The current character
	L   Location // Line
	C   rune     // Character
	EOF bool     // true if current C and L are invalid, at end of input

	// The lookahead (next) character
	NL   Location
	NC   rune
	NEOF bool // true if current NC and NL are invalid, will be at end of input with next advance
}

// NewCharReader returns a new CharReader with the input preadvanced so that all fields are valid.
func NewCharReader(source string, line uint) *CharReader {
	return NewRawCharReader(strings.NewReader(source), line)
}

// NewRawCharReader returns a new CharReader with the input preadvanced so that all fields are valid.
func NewRawCharReader(source io.RuneReader, line uint) *CharReader {
	cr := new(CharReader)

	cr.source = source

	cr.L = Location(0).L(uint64(line))
	cr.NL = Location(0).L(uint64(line))

	// prime the pump
	cr.Next()
	cr.Next()

	return cr
}

// Match returns true if C matches one of the chars in the string.
func (cr *CharReader) Match(chars string) bool {
	if cr.EOF {
		return false
	}

	for _, char := range chars {
		if cr.C == char {
			return true
		}
	}
	return false
}

// NMatch returns true if NC matches one of the chars in the string.
func (cr *CharReader) NMatch(chars string) bool {
	if cr.NEOF {
		return false
	}

	for _, char := range chars {
		if cr.NC == char {
			return true
		}
	}
	return false
}

// MatchAlpha returns true if C is an underscore or unicode letter
func (cr *CharReader) MatchAlpha() bool {
	if cr.EOF {
		return false
	}

	return cr.C == '_' || unicode.IsLetter(cr.C)
}

// MatchNumeric returns true if C is a decimal digit.
func (cr *CharReader) MatchNumeric() bool {
	if cr.EOF {
		return false
	}
	// It would be WAY too complicated to support non-arabic numerals.
	return cr.C >= '0' && cr.C <= '9'
}

// Next advances the reader one character position.
// C, L, and EOF gain the previous values of NC, NL, and NEOF, additionally a newly read character becomes NC.
// All carriage returns are simply ignored, and newlines will advance the current value of NL.
// If the end of input is reached NEOF is set to true.
func (cr *CharReader) Next() {
	if cr.EOF {
		return
	}
	if cr.NEOF {
		cr.EOF = true
		return
	}

	var err error

	cr.C = cr.NC
	cr.L = cr.NL

again:
	cr.NC, _, err = cr.source.ReadRune() // err should only ever be io.EOF
	if err != nil {
		cr.NEOF = true
		return
	}
	cr.NL = cr.NL.CPlus()

	// We simply strip carriage returns.
	if cr.NC == '\r' {
		// This isn't a loop because this is an exception and it looks friggin weird to wrap
		// the whole thing in a loop for what amounts to an error case.
		goto again
	}

	if cr.NC == '\n' {
		cr.NL = cr.NL.LPlus().C(0)
		return
	}
}

// Eat the given characters until something else is found or EOF.
func (cr *CharReader) Eat(chars string) {
	for cr.Match(chars) {
		cr.Next()
		if cr.EOF {
			return
		}
	}
}

// Eat all characters until one of the given chars are found or EOF.
func (cr *CharReader) EatUntil(chars string) {
	for !cr.Match(chars) {
		cr.Next()
		if cr.EOF {
			return
		}
	}
}

// ReadMatch reads all matching characters into a buffer until a nonmatching character is found or EOF.
func (cr *CharReader) ReadMatch(chars string, buf []rune) []rune {
	for cr.Match(chars) {
		buf = append(buf, cr.C)
		cr.Next()
		if cr.EOF {
			return buf
		}
	}
	return buf
}

// ReadMatchLimit reads all matching characters into a buffer until a nonmatching character is found,
// the limit is reached, or EOF. Returns true if the read stopped due to the limit.
func (cr *CharReader) ReadMatchLimit(chars string, buf []rune, limit int) (bool, []rune) {
	i := 0
	for cr.Match(chars) && i < limit {
		buf = append(buf, cr.C)
		cr.Next()
		if cr.EOF {
			return false, buf
		}
		i++
	}

	return i == limit, buf
}

// ReadUntil reads all characters into a buffer until a matching character is found or EOF.
func (cr *CharReader) ReadUntil(chars string, buf []rune) []rune {
	for !cr.Match(chars) {
		buf = append(buf, cr.C)
		cr.Next()
		if cr.EOF {
			return buf
		}
	}
	return buf
}

// Location represents a line and column number for a given character in the lexer input.
type Location uint64

// Line returns a 48 bit line number.
func (l Location) Line() uint64 {
	return uint64((l & 0x0000ffffffffffff))
}

// Column returns a 16 bit column number.
func (l Location) Column() uint16 {
	return uint16((l & 0xffff000000000000) >> 48)
}

func (l Location) String() string {
	return fmt.Sprintf("%v:%v", l.Line(), l.Column())
}

// L is a composite constructor for a location, setting the line part. If you pass in an integer that is too
// large to fit the 48 bit storage area, 0 will be used instead.
func (l Location) L(i uint64) Location {
	if uint64(i)&0xffff000000000000 != 0 {
		i = 0
	}
	l = l & 0xffff000000000000
	l = l | Location(i)
	return l
}

// C is a composite constructor for a location, setting the column part. If you pass in an integer that is
// too large to fit the 16 bit storage area, 0 will be used instead.
func (l Location) C(i uint16) Location {
	l = l & 0x0000ffffffffffff
	l = l | (Location(i) << 48)
	return l
}

// LPlus increments the line portion of a Location and returns the result.
func (l Location) LPlus() Location {
	i := l.Line()
	return l.L(i + 1)
}

// CPlus increments the column portion of a Location and returns the result.
func (l Location) CPlus() Location {
	i := l.Column()
	return l.C(i + 1)
}
