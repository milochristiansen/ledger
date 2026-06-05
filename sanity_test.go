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

package ledger_test

import (
	"testing"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse"
)

var TestBasicFunctionInput = `
account Expenses:Food
	note Test Account

2012-03-10 * TesT
	; Example
	; :Tag1:Tag2:
	; Key: Value
    Expenses:Food       $20.00
    Assets:C a s h             = $5.25 ; Poor wallet :(
`

// This is a simple sanity check that makes sure the base features are functional under normal conditions.
// I do not test nearly every case here, this is just to catch major errors.
func TestBasicFunction(t *testing.T) {
	f, err := parse.ParseLedger(parse.NewCharReader(TestBasicFunctionInput, 33))
	if err != nil {
		t.Error(err)
	}

	transactions, directives := f.Transactions(), f.Directives()

	if len(directives) != 1 {
		t.Fatalf("Incorrect number of directives: %v", len(directives))
	}

	dr := directives[0]

	if dr.Type != "account" {
		t.Errorf("Incorrect directive type: %v", dr.Type)
	}

	if dr.Argument != "Expenses:Food" {
		t.Errorf("Incorrect directive argument: %v", dr.Argument)
	}

	if len(dr.Lines) != 1 {
		t.Fatalf("Incorrect number of directive lines: %v", len(dr.Lines))
	}

	if dr.Lines[0] != "note Test Account" {
		t.Errorf("Incorrect directive line: %v", dr.Lines[0])
	}

	if len(transactions) != 1 {
		t.Fatalf("Incorrect number of transactions: %v", len(transactions))
	}

	tr := transactions[0]

	if tr.Description != "TesT" {
		t.Errorf("Incorrect description: %v", tr.Description)
	}

	if tr.Status != ledger.StatusClear {
		t.Errorf("Incorrect clear status: %v", tr.Status)
	}

	if len(tr.Postings) != 2 {
		t.Fatalf("Incorrect number of postings: %v", len(tr.Postings))
	}

	if tr.Postings[0].Account != "Expenses:Food" {
		t.Errorf("Incorrect posting 0 account: %v", tr.Postings[0].Account)
	}
	if tr.Postings[0].Note != "" {
		t.Errorf("Incorrect posting 0 note: %v", tr.Postings[0].Note)
	}
	if tr.Postings[0].Status != ledger.StatusUndefined {
		t.Errorf("Incorrect posting 0 status: %v", tr.Postings[0].Status)
	}
	if tr.Postings[0].Value != 200000 {
		t.Errorf("Incorrect posting 0 value: %v", tr.Postings[0].Value)
	}
	if tr.Postings[0].Null {
		t.Errorf("Posting 0 incorrectly marked null.")
	}
	if tr.Postings[0].HasAssert {
		t.Errorf("Posting 0 incorrectly marked as containing an assertion.")
	}
	if tr.Postings[0].Assert != 0 {
		t.Errorf("Posting 0 has invalid assertion amount: %v", tr.Postings[1].Assert)
	}

	if tr.Postings[1].Account != "Assets:C a s h" {
		t.Errorf("Incorrect posting 1 account: %v", tr.Postings[1].Account)
	}
	if tr.Postings[1].Note != "Poor wallet :(" {
		t.Errorf("Incorrect posting 1 note: %v", tr.Postings[1].Note)
	}
	if tr.Postings[1].Status != ledger.StatusUndefined {
		t.Errorf("Incorrect posting 1 status: %v", tr.Postings[1].Status)
	}
	if tr.Postings[1].Value != 0 {
		t.Errorf("Incorrect posting 1 value: %v", tr.Postings[1].Value)
	}
	if !tr.Postings[1].Null {
		t.Errorf("Posting 1 incorrectly marked not null.")
	}
	if !tr.Postings[1].HasAssert {
		t.Errorf("Posting 1 incorrectly marked as not containing an assertion.")
	}
	if tr.Postings[1].Assert != 52500 {
		t.Errorf("Posting 1 has invalid assertion amount: %v", tr.Postings[1].Assert)
	}

	if len(tr.Comments) != 1 {
		t.Fatalf("Incorrect number of comments: %v", len(tr.Comments))
	}
	if tr.Comments[0] != "Example" {
		t.Errorf("Incorrect comment value: %v", tr.Comments[0])
	}

	if len(tr.Tags) != 2 {
		t.Fatalf("Incorrect number of tags: %v", len(tr.Tags))
	}
	if !tr.Tags["Tag1"] || !tr.Tags["Tag2"] {
		t.Errorf("Incorrect tags: %#v", tr.Tags)
	}

	if len(tr.KVPairs) != 1 {
		t.Fatalf("Incorrect number of k/v pairs: %v", len(tr.KVPairs))
	}
	if tr.KVPairs["Key"] != "Value" {
		t.Errorf("Incorrect k/v values: %#v", tr.Tags)
	}

	ok, ac := tr.Balance()
	if !ok {
		t.Errorf("Transaction does not balance.")
	}
	if len(ac) != 2 {
		t.Fatalf("Incorrect balance report length: %v", len(ac))
	}
	if ac["Expenses:Food"] != 200000 {
		t.Errorf("Incorrect balance report value for Expenses:Food: %v", ac["Expenses:Food"])
	}
	if ac["Assets:C a s h"] != -200000 {
		t.Errorf("Incorrect balance report value for Assets:C a s h: %v", ac["Assets:C a s h"])
	}

}
