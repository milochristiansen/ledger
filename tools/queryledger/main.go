package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/milochristiansen/ledger/tools"
)

var (
	date         = flag.String("date", "", "date or range (YYYY/MM/DD or YYYY/MM/DD:YYYY/MM/DD)")
	acct         = flag.String("account", "", "only transactions with a posting matching this regex")
	excludeAcct  = flag.String("exclude-account", "", "exclude transactions with a posting matching this regex")
	payee        = flag.String("payee", "", "only transactions whose description matches this regex")
	excludePayee = flag.String("exclude-payee", "", "exclude transactions whose description matches this regex")
	statusFl     = flag.String("status", "", "only transactions with this status (clear/*, pending/!, none)")
	excludeStFl  = flag.String("exclude-status", "", "exclude transactions with this status")
	amount       = flag.String("amount", "", "exact amount or range ($20.00 or $10.00:$30.00)")
	tagFl        = flag.String("tag", "", "only transactions with this tag")
	kvFl         = flag.String("kv", "", "only transactions with this key:value pair (or just key)")
	asJSON       = flag.Bool("json", false, "output in JSON format")
	refFlag      = flag.String("ref", "", "find a single transaction by its ref code")
	csvFields    = flag.String("csv", "", "output as TSV with comma-separated fields (date, description, account, amount, ..., comment, tag:NAME, kv:KEY)")
	fileFlag     = flag.String("file", "", "limit -ref search to this file (falls back to all files)")
	refWasSet    bool
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: queryledger [flags] <ledger-file>\n"+
			"\nquery mode:\n"+
			"  -date YYYY/MM/DD or YYYY/MM/DD:YYYY/MM/DD    filter by date (or range)\n"+
			"  -account REGEX                                 include postings matching account\n"+
			"  -exclude-account REGEX                          exclude postings matching account\n"+
			"  -payee REGEX                                   include matching descriptions\n"+
			"  -exclude-payee REGEX                            exclude matching descriptions\n"+
			"  -status clear|pending|none|*|!                   filter by transaction status\n"+
			"  -exclude-status clear|pending|none|*|!            exclude transaction status\n"+
			"  -amount $X.XX or $X:$Y                          exact amount or range\n"+
			"  -tag TAG                                        only transactions with this tag\n"+
			"  -kv KEY or KEY:VALUE                            only transactions with this key:value pair\n"+
			"\nref lookup mode:\n"+
			"  -ref INDEX:HASH                                 find by ref code\n"+
			"  -file FILENAME                                  limit ref search to file\n"+
			"\noutput:\n"+
			"  -json                                           JSON output\n"+
			"  -csv date,description,account,amount,note,...   TSV with named columns\n"+
			"  (default)                                       ledger format with file:line header\n"+
			"\nCSV fields: date, description, account, account:N, amount, amount:N,\n"+
			"  note, note:N, status:N, assert, assert:N, file, line, ref, status, code, clear_date,\n"+
			"  comment, tag:NAME, kv:KEY\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "ref" {
			refWasSet = true
		}
	})
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	if refWasSet {
		if *refFlag == "" {
			fmt.Fprintf(os.Stderr, "error: -ref requires a value\n")
			os.Exit(2)
		}
		result, err := tools.QueryByRef(flag.Arg(0), *refFlag, *fileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if result != nil {
			outputResult(result)
		}
		return
	}
	results, err := tools.Query(flag.Arg(0), tools.QueryParams{
		Date:           *date,
		Account:        *acct,
		ExcludeAccount: *excludeAcct,
		Payee:          *payee,
		ExcludePayee:   *excludePayee,
		Amount:         *amount,
		Status:         *statusFl,
		ExcludeStatus:  *excludeStFl,
		Tag:            *tagFl,
		KV:             *kvFl,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if *asJSON {
		outputJSON(results)
	} else if *csvFields != "" {
		outputCSV(results, *csvFields)
	} else {
		outputLedger(results)
	}
}

func outputResult(r *tools.QueryResult) {
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r)
	} else if *csvFields != "" {
		outputCSV([]tools.QueryResult{*r}, *csvFields)
	} else {
		outputLedger([]tools.QueryResult{*r})
	}
}

func outputJSON(results []tools.QueryResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, r := range results {
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding output: %v\n", err)
			os.Exit(1)
		}
	}
}

func outputLedger(results []tools.QueryResult) {
	for _, r := range results {
		fmt.Printf("; %s:%d, ref: %s\n\n%s\n", r.File, r.Entry.Location.Line(), r.Ref, r.Entry.String())
	}
}

func outputCSV(results []tools.QueryResult, fields string) {
	names := strings.Split(fields, ",")
	for i, n := range names {
		names[i] = strings.TrimSpace(n)
	}
	for _, r := range results {
		values := make([]string, len(names))
		for i, name := range names {
			values[i] = tools.CSVField(name, &r)
		}
		fmt.Println(strings.Join(values, "\t"))
	}
}
