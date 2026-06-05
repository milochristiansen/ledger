/*
Copyright 2022 by Samuel Loewen

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

package main

import (
	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/tools"
)

var usage string = `Usage:

Replace accounts in postings using rules from a matcher file. Only matched transactions will
be output.
`

func main() {
	fs := tools.CommonFlagSet(tools.FlagSourceFile|tools.FlagDestFile|tools.FlagMatchFile|tools.FlagAccountName, usage)
	fs.Parse()

	src := tools.LoadLedgerFile(fs.SourceFile)

	matchers := tools.LoadMatchFile(fs.MatchFile)
	matched := tools.MatchedTransactions(src, fs.AccountName, matchers)
	entries := make([]ledger.Entry, len(matched))
	for i := range matched {
		t := matched[i]
		entries[i] = &t
	}
	dst := &ledger.File{Entries: entries}

	tools.WriteLedgerFile(fs.DestFile, dst)
}
