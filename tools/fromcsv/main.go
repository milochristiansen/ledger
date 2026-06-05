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
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/tools"
)

var usage string = `Usage: fromcsv [-o <dest>]|[-output <dest>] options... <src>

Converts a CSV file to a ledger file.

	-h, -help 
		Show this help.
	-o, -output <file> (default stdout)
		Write transactions to this file
	-datefmt <date> (default 01/02/2006)
		Use an example date for Mon Jan 2, 2006 3:04:05 PM to specify the
		date format to parse from.
	-date <name> (default date)
		This argument specifies which field contains the date. The header
		will be used to find the field. If -noheader is specified, then
		the value must be the index of the field.
	-amount <name> (default amount)
		This argument specifies which field contains the amount. The header
		will be used to find the field. If -noheader is specified, then
		the value must be the index of the field.
	-desc <name> (default desc)
		This argument specifies which field contains the desciption. The header
		will be used to find the field. If -noheader is specified, then
		the value must be the index of the field. This argument may be
		provided multiple times to concatonate the values of several fields
		for the description.
	-from <account> (default Account:From)
		Positive amounts will take from this account
	-to <account> (default Account:To)
		Positive amounts will add to this account
`

var output string
var noHeader bool

var dateFmt string
var dateField string
var descField map[string]bool = map[string]bool{}
var amountField string

var accountFrom string
var accountTo string

var dateFieldIx int = -1
var descFieldIx map[int]bool = map[int]bool{}
var amountFieldIx int = -1

var help bool

func main() {
	flag.StringVar(&output, "output", "-", "file to write csv to")
	flag.StringVar(&output, "o", "-", "file to write csv to")
	flag.BoolVar(&noHeader, "noheader", false, "the csv doesn't contain any header")
	flag.StringVar(&dateFmt, "datefmt", "01/02/2006", "Jan 2, 2006 at 3:04:05 PM in expected date format")
	flag.StringVar(&dateField, "date", "date", "name of date field")
	flag.StringVar(&amountField, "amount", "amount", "name of amount field")
	flag.StringVar(&accountFrom, "from", "Account:From", "positive amounts take money from this account")
	flag.StringVar(&accountTo, "to", "Account:To", "positive amounts add money to this account")
	flag.BoolVar(&help, "help", false, "show this help")
	flag.BoolVar(&help, "h", false, "show this help")
	flag.Func("desc", "name of description field", func(arg string) error {
		descField[arg] = true
		return nil
	})
	flag.Parse()
	if help {
		fmt.Print(usage)
		os.Exit(0)
	}

	input := flag.Arg(0)
	var inFile, outFile *os.File
	var err error
	if input == "" || input == "-" {
		inFile = os.Stdin
	} else {
		inFile, err = os.Open(input)
		defer inFile.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open input file: %v\n", err)
			os.Exit(1)
		}
	}
	if output == "-" {
		outFile = os.Stdout
	} else {
		outFile, err = os.Create(output)
		defer outFile.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open output file: %v\n", err)
			os.Exit(1)
		}
	}

	reader := csv.NewReader(inFile)

	if !noHeader {
		header, err := reader.Read()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read header: %v\n", err)
			os.Exit(1)
		}

		for i, field := range header {
			if field == dateField {
				dateFieldIx = i
			}

			if field == amountField {
				amountFieldIx = i
			}

			if descField[field] {
				descFieldIx[i] = true
			}
		}
	} else {
		dateFieldIx, err = strconv.Atoi(dateField)
		if err != nil {
			fmt.Fprintln(os.Stderr, "-date argument is not a number")
			os.Exit(2)
		}

		amountFieldIx, err = strconv.Atoi(amountField)
		if err != nil {
			fmt.Fprintln(os.Stderr, "-amount argument is not a number")
			os.Exit(2)
		}

		for desc, has := range descField {
			if !has {
				continue
			}

			descIx, err := strconv.Atoi(desc)
			if err != nil {
				fmt.Fprintln(os.Stderr, "-desc argument is not a number")
				os.Exit(2)
			}

			descFieldIx[descIx] = true
		}

	}

	if dateFieldIx == -1 {
		fmt.Fprintln(os.Stderr, "date field not found or specified")
		os.Exit(2)
	}

	if amountFieldIx == -1 {
		fmt.Fprintln(os.Stderr, "amount field not found or specified")
		os.Exit(2)
	}

	hasDesc := false
	for _, has := range descFieldIx {
		hasDesc = hasDesc || has
	}

	if !hasDesc {
		fmt.Fprintln(os.Stderr, "desc field not found or specified")
		os.Exit(2)
	}

	minLen := dateFieldIx
	if amountFieldIx > minLen {
		minLen = amountFieldIx
	}
	for desc, has := range descFieldIx {
		if !has {
			continue
		}

		if desc > minLen {
			minLen = desc
		}
	}
	minLen++

	trs := []ledger.Transaction{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read input record: %v\n", err)
			os.Exit(1)
		}

		if len(record) < minLen {
			fmt.Fprintln(os.Stderr, "found input record with too few fields")
			os.Exit(3)
		}

		date, err := time.Parse(dateFmt, record[dateFieldIx])
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse date: %s\n", record[dateFieldIx])
			os.Exit(3)
		}

		amountStr := record[amountFieldIx]
		amountClean := strings.Builder{}
		negate := false
		for _, chr := range amountStr {
			switch chr {
			case '$':
				// eat all $
			case '(':
				negate = true
			case ')':
				// eat all )
			case ',':
				// eat all ,
			default:
				amountClean.WriteRune(chr)
			}
		}
		amount, err := ledger.ParseValueNumber(amountClean.String())
		if negate {
			amount = -amount
		}

		desc := make([]string, 0, len(descFieldIx))
		for descIx, has := range descFieldIx {
			if !has {
				continue
			}

			desc = append(desc, record[descIx])
		}

		tr := ledger.Transaction{
			Description: strings.Join(desc, " "),
			Date:        date,
			Status:      ledger.StatusClear,
			KVPairs: map[string]string{
				"ID":  <-tools.IDService,
				"RID": <-tools.IDService,
			},
			Postings: []ledger.Posting{
				{
					Account: accountTo,
					Value:   amount,
				},
				{
					Account: accountFrom,
					Null:    true,
				},
			},
		}
		trs = append(trs, tr)
	}

	entries := make([]ledger.Entry, len(trs))
	for i := range trs {
		t := trs[i]
		entries[i] = &t
	}
	err = (&ledger.File{Entries: entries}).Format(outFile)
}
