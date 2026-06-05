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

package tools

import "github.com/milochristiansen/ledger"

// LTail tails a ledger file based on a ID and RID. There are no error cases
// (if the ID doesn't exist you just get an empty file).
func LTail(f *ledger.File, id, rid string) *ledger.File {
	// Find the transaction with the given ID/RID, searching in reverse.
	cutAfter := -1
	for i := len(f.Entries) - 1; i >= 0; i-- {
		tp, ok := f.Entries[i].(*ledger.Transaction)
		if !ok {
			continue
		}
		if fid, ok := tp.KVPairs["ID"]; ok && fid == id {
			if rid != "" {
				if frid, ok := tp.KVPairs["RID"]; ok && frid == rid {
					cutAfter = i
					break
				}
				continue
			}
			cutAfter = i
			break
		}
	}

	// Everything from cutAfter onward (inclusive) is kept.
	var entries []ledger.Entry
	if cutAfter >= 0 {
		entries = make([]ledger.Entry, len(f.Entries)-cutAfter)
		copy(entries, f.Entries[cutAfter:])
	}

	return &ledger.File{Entries: entries}
}
