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

2. Altered source versions must be plainly marked as, and must not be
misrepresented as being the original software.

3. This notice may not be removed or altered from any source distribution.
*/

package tools

import (
	"errors"

	"github.com/milochristiansen/ledger"
)

// Zipper takes two ledger files and zips them together in a deterministic manner.
// On error os.Exit is called and the error is logged to standard error.
// All directives are deduplicated and moved to the top of the file.
func Zipper(a *ledger.File, b *ledger.File) *ledger.File {
	return HandleErrV(ZipperHTTP(a, b))
}

// ZipperHTTP is like Zipper, but intended for use in HTTP handlers and the like
// where the standard command error handling is not desirable.
func ZipperHTTP(a *ledger.File, b *ledger.File) (*ledger.File, error) {
	// Deduplicate directives
	ad := a.Directives()
	bd := b.Directives()
	drs := make([]ledger.Directive, len(ad))
	copy(drs, ad)
outer:
	for _, d2 := range bd {
		for _, d1 := range ad {
			if d2.Compare(d1) {
				continue outer
			}
		}
		drs = append(drs, d2)
	}

	// Merge transactions
	at := a.Transactions()
	bt := b.Transactions()
	trs := []ledger.Transaction{}

	// First, zoom through the master file until we find the sync point.
	syncPoint := len(at) - 1
	for ; syncPoint >= 0; syncPoint-- {
		if at[syncPoint].Code == bt[0].Code {
			break
		}
	}
	if syncPoint == len(at) {
		return nil, errors.New("No sync point found!")
	}

	// Add transactions from the master up to the sync point
	for i := 0; i <= syncPoint; i++ {
		trs = append(trs, at[i])
	}

	// Now continue adding from the master up until the last transaction that matches.
	i1, i2 := syncPoint+1, 1
	for i1 < len(at) || i2 < len(bt) {
		if at[i1].Code != bt[i2].Code {
			break
		}
		trs = append(trs, at[i1])
		i1++
		i2++
	}

	// Now zipper the differences together from the last sync point
	for i1 < len(at) || i2 < len(bt) {
		// If only one side is left, just append it and bail.
		if i1 >= len(at) {
			trs = append(trs, bt[i2])
			i2++
			continue
		}
		if i2 >= len(bt) {
			trs = append(trs, at[i1])
			i1++
			continue
		}

		// If there is a clear difference between the times, the earlier one goes first.
		if at[i1].Date.Before(bt[i2].Date) {
			trs = append(trs, at[i1])
			i1++
			continue
		}
		if at[i1].Date.After(bt[i2].Date) {
			trs = append(trs, bt[i2])
			i2++
			continue
		}

		// if the times are the same, try to order lexically by ID to preserve determinism.
		dir := chooseAB(at[i1].KVPairs, bt[i2].KVPairs, "ID")
		if dir < 0 {
			trs = append(trs, at[i1])
			i1++
			continue
		}
		if dir > 0 {
			trs = append(trs, bt[i2])
			i2++
			continue
		}

		// try the revision id
		dir = chooseAB(at[i1].KVPairs, bt[i2].KVPairs, "RID")
		if dir < 0 {
			trs = append(trs, at[i1])
			i1++
			continue
		}
		if dir > 0 {
			trs = append(trs, bt[i2])
			i2++
			continue
		}

		// nothing matters, just pick one
		trs = append(trs, at[i1])
		i1++
		continue
	}

	// Build entries: directives first, then transactions
	entries := make([]ledger.Entry, 0, len(drs)+len(trs))
	for i := range drs {
		d := drs[i]
		entries = append(entries, &d)
	}
	for i := range trs {
		t := trs[i]
		entries = append(entries, &t)
	}
	return &ledger.File{Entries: entries}, nil
}

// -1 == a, 0 == neither, 1 == b
func chooseAB(a, b map[string]string, key string) int {
	av, aok := a[key]
	bv, bok := b[key]
	if !aok && !bok {
		return 0
	}
	if !aok && bok {
		return 1
	}
	if aok && !bok {
		return -1
	}
	if av == bv {
		return 0
	}
	if av < bv {
		return -1
	}
	return 1
}
