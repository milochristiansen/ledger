/*
Copyright 2022 by Milo Christiansen

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
	"os"

	"github.com/milochristiansen/ledger/tools"
)

func main() {
	fs := tools.CommonFlagSet(tools.FlagDestFile|tools.FlagSourceFile|tools.FlagAccountName|tools.FlagMatchFile, usage)
	fs.Parse()

	fr := tools.HandleErrV(os.Open(fs.SourceFile))
	matchers := []tools.Matcher{}
	if fs.MatchFile != "" {
		matchers = tools.LoadMatchFile(fs.MatchFile)
	}

	// Load OFX file
	f := tools.FromOFX(fr, fs.AccountName, matchers)

	tools.WriteLedgerFile(fs.DestFile, f)
}

var usage = `Usage:

This program takes an OFX file and converts it to a ledger file.
`
