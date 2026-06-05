package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse"
	"github.com/milochristiansen/ledger/tools"
)

type queryResult struct {
	File  string              `json:"file"`
	Ref   string              `json:"ref"`
	Entry *ledger.Transaction `json:"entry"`
}

var (
	date         = flag.String("date", "", "date or range (YYYY/MM/DD or YYYY/MM/DD:YYYY/MM/DD)")
	acct         = flag.String("account", "", "only transactions with a posting matching this regex")
	excludeAcct  = flag.String("exclude-account", "", "exclude transactions with a posting matching this regex")
	payee        = flag.String("payee", "", "only transactions whose description matches this regex")
	excludePayee = flag.String("exclude-payee", "", "exclude transactions whose description matches this regex")
	amount       = flag.String("amount", "", "exact amount or range ($20.00 or $10.00:$30.00)")
	asJSON       = flag.Bool("json", false, "output in JSON format")
	csvFields    = flag.String("csv", "", "output as TSV with comma-separated fields (date, description, account, amount, account:N, amount:N, file, line, ref, status, code, clear_date)")
	refFlag      = flag.String("ref", "", "find a single transaction by its ref code")
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
			"  -amount $X.XX or $X:$Y                          exact amount or range\n"+
			"\nref lookup mode:\n"+
			"  -ref INDEX:HASH                                 find by ref code\n"+
			"  -file FILENAME                                  limit ref search to file\n"+
			"\noutput:\n"+
			"  -json                                           JSON output\n"+
			"  -csv date,description,account,amount,note,...   TSV with named columns\n"+
			"  (default)                                       ledger format with file:line header\n"+
			"\nCSV fields: date, description, account, account:N, amount, amount:N,\n"+
			"  note, note:N, status:N, assert, assert:N, file, line, ref, status, code, clear_date\n")
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
		result, err := findByRef(flag.Arg(0), *refFlag, *fileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if result != nil {
			outputResult(result)
		}
		return
	}
	results, err := query(flag.Arg(0))
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

func outputResult(r *queryResult) {
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r)
	} else if *csvFields != "" {
		outputCSV([]queryResult{*r}, *csvFields)
	} else {
		outputLedger([]queryResult{*r})
	}
}

func outputJSON(results []queryResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, r := range results {
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding output: %v\n", err)
			os.Exit(1)
		}
	}
}

func outputLedger(results []queryResult) {
	for _, r := range results {
		fmt.Printf("; %s:%d, ref: %s\n\n%s\n", r.File, r.Entry.Location.Line(), r.Ref, r.Entry.String())
	}
}

func outputCSV(results []queryResult, fields string) {
	names := strings.Split(fields, ",")
	for i, n := range names {
		names[i] = strings.TrimSpace(n)
	}
	for _, r := range results {
		t := r.Entry
		values := make([]string, len(names))
		for i, name := range names {
			values[i] = fieldValue(name, r.File, r.Ref, t)
		}
		fmt.Println(strings.Join(values, "\t"))
	}
}

type fileEntry struct {
	path string
	f    *ledger.File
}

func findByRef(rootPath, ref, scopeFile string) (*queryResult, error) {
	w, err := tools.NewFileSafeWriter(rootPath)
	if err != nil {
		return nil, err
	}
	pis, err := w.Includes(w.Add)
	if err != nil {
		return nil, fmt.Errorf("loading includes: %w", err)
	}
	files := []fileEntry{{filepath.Base(rootPath), w.File}}
	for _, pi := range pis {
		if pi.File != nil {
			files = append(files, fileEntry{pi.Path, pi.File})
		}
	}
	colon := strings.LastIndex(ref, ":")
	if colon < 0 {
		return nil, fmt.Errorf("invalid ref: %s", ref)
	}
	index, err := strconv.Atoi(ref[:colon])
	if err != nil {
		return nil, fmt.Errorf("invalid ref index: %s", ref)
	}
	hash := ref[colon+1:]
	if scopeFile != "" {
		for _, fe := range files {
			if fe.path == scopeFile {
				if r := searchFile(fe, index, hash); r != nil {
					return r, nil
				}
				break
			}
		}
		return nil, nil
	}
	for _, fe := range files {
		if r := searchFile(fe, index, hash); r != nil {
			return r, nil
		}
	}
	return nil, nil
}

func searchFile(fe fileEntry, index int, hash string) *queryResult {
	n := len(fe.f.Entries)
	for radius := 0; radius <= n; radius++ {
		for _, offset := range [2]int{index - radius, index + radius} {
			if offset < 0 || offset >= n {
				continue
			}
			t, ok := fe.f.Entries[offset].(*ledger.Transaction)
			if !ok {
				continue
			}
			tc := *t.CleanCopy()
			if makeRef(fe.path, offset, &tc) == hash {
				return &queryResult{
					File:  fe.path,
					Ref:   fmt.Sprintf("%d:%s", offset, hash),
					Entry: &tc,
				}
			}
		}
	}
	return nil
}

func fieldValue(name, file, ref string, t *ledger.Transaction) string {
	switch {
	case name == "file":
		return file
	case name == "line":
		return strconv.FormatUint(t.Location.Line(), 10)
	case name == "ref":
		return ref
	case name == "date":
		return t.Date.Format("2006/01/02")
	case name == "clear_date":
		if t.ClearDate.IsZero() {
			return ""
		}
		return t.ClearDate.Format("2006/01/02")
	case name == "status":
		switch t.Status {
		case ledger.StatusClear:
			return "*"
		case ledger.StatusPending:
			return "!"
		default:
			return ""
		}
	case name == "code":
		return t.Code
	case name == "description":
		return t.Description
	case strings.HasPrefix(name, "account"):
		ix := 0
		if rest := strings.TrimPrefix(name, "account"); rest != "" {
			ix, _ = strconv.Atoi(strings.TrimPrefix(rest, ":"))
		}
		if ix >= 0 && ix < len(t.Postings) {
			return t.Postings[ix].Account
		}
		return ""
	case strings.HasPrefix(name, "amount"):
		ix := 0
		if rest := strings.TrimPrefix(name, "amount"); rest != "" {
			ix, _ = strconv.Atoi(strings.TrimPrefix(rest, ":"))
		}
		if ix >= 0 && ix < len(t.Postings) && !t.Postings[ix].Null {
			return ledger.FormatValueNumber(t.Postings[ix].Value)
		}
		return ""
	case strings.HasPrefix(name, "note"):
		ix := 0
		if rest := strings.TrimPrefix(name, "note"); rest != "" {
			ix, _ = strconv.Atoi(strings.TrimPrefix(rest, ":"))
		}
		if ix >= 0 && ix < len(t.Postings) {
			return t.Postings[ix].Note
		}
		return ""
	case strings.HasPrefix(name, "status"):
		ix := 0
		if rest := strings.TrimPrefix(name, "status"); rest != "" {
			ix, _ = strconv.Atoi(strings.TrimPrefix(rest, ":"))
		}
		if ix >= 0 && ix < len(t.Postings) {
			switch t.Postings[ix].Status {
			case ledger.StatusClear:
				return "*"
			case ledger.StatusPending:
				return "!"
			default:
				return ""
			}
		}
		return ""
	case strings.HasPrefix(name, "assert"):
		ix := 0
		if rest := strings.TrimPrefix(name, "assert"); rest != "" {
			ix, _ = strconv.Atoi(strings.TrimPrefix(rest, ":"))
		}
		if ix >= 0 && ix < len(t.Postings) && t.Postings[ix].HasAssert {
			return ledger.FormatValueNumber(t.Postings[ix].Assert)
		}
		return ""
	}
	return ""
}

func parseAmountFlag(s string) (int64, error) {
	v, null, err := parse.ReadAmount(parse.NewCharReader(s+"\n", 1))
	if err != nil {
		return 0, err
	}
	if null {
		return 0, fmt.Errorf("amount must not be empty")
	}
	return v, nil
}

func parseDateFlag(s string) (after, before time.Time, err error) {
	if idx := strings.Index(s, ":"); idx >= 0 {
		after, err = time.Parse("2006/01/02", s[:idx])
		if err != nil {
			return after, before, fmt.Errorf("date range start: %w", err)
		}
		before, err = time.Parse("2006/01/02", s[idx+1:])
		if err != nil {
			return after, before, fmt.Errorf("date range end: %w", err)
		}
		if after.After(before) {
			after, before = before, after
		}
	} else {
		after, err = time.Parse("2006/01/02", s)
		if err != nil {
			return after, before, fmt.Errorf("date: %w", err)
		}
		before = after
	}
	before = before.Add(24 * time.Hour)
	return after, before, nil
}

func query(rootPath string) ([]queryResult, error) {
	w, err := tools.NewFileSafeWriter(rootPath)
	if err != nil {
		return nil, err
	}
	pis, err := w.Includes(w.Add)
	if err != nil {
		return nil, fmt.Errorf("loading includes: %w", err)
	}
	files := []fileEntry{{filepath.Base(rootPath), w.File}}
	for _, pi := range pis {
		if pi.File != nil {
			files = append(files, fileEntry{pi.Path, pi.File})
		}
	}
	var (
		acctRE         *regexp.Regexp
		excludeAcctRE  *regexp.Regexp
		payeeRE        *regexp.Regexp
		excludePayeeRE *regexp.Regexp
		afterD         time.Time
		beforeD        time.Time
		hasDate        bool
	)
	if *date != "" {
		afterD, beforeD, err = parseDateFlag(*date)
		if err != nil {
			return nil, err
		}
		hasDate = true
	}
	if *acct != "" {
		acctRE, err = regexp.Compile(*acct)
		if err != nil {
			return nil, fmt.Errorf("account regex: %w", err)
		}
	}
	if *excludeAcct != "" {
		excludeAcctRE, err = regexp.Compile(*excludeAcct)
		if err != nil {
			return nil, fmt.Errorf("exclude-account regex: %w", err)
		}
	}
	if *payee != "" {
		payeeRE, err = regexp.Compile(*payee)
		if err != nil {
			return nil, fmt.Errorf("payee regex: %w", err)
		}
	}
	if *excludePayee != "" {
		excludePayeeRE, err = regexp.Compile(*excludePayee)
		if err != nil {
			return nil, fmt.Errorf("exclude-payee regex: %w", err)
		}
	}
	var exactAmt, minVal, maxVal int64
	hasAmt := *amount != ""
	isRange := hasAmt && strings.Contains(*amount, ":")
	if hasAmt {
		if isRange {
			parts := strings.SplitN(*amount, ":", 2)
			minVal, err = parseAmountFlag(parts[0])
			if err != nil {
				return nil, fmt.Errorf("amount min: %w", err)
			}
			maxVal, err = parseAmountFlag(parts[1])
			if err != nil {
				return nil, fmt.Errorf("amount max: %w", err)
			}
			if minVal > maxVal {
				minVal, maxVal = maxVal, minVal
			}
		} else {
			exactAmt, err = parseAmountFlag(*amount)
			if err != nil {
				return nil, fmt.Errorf("amount: %w", err)
			}
		}
	}
	var results []queryResult
	for _, fe := range files {
		for i, ent := range fe.f.Entries {
			t, ok := ent.(*ledger.Transaction)
			if !ok {
				continue
			}
			if hasDate && (t.Date.Before(afterD) || !t.Date.Before(beforeD)) {
				continue
			}
			if acctRE != nil {
				found := false
				for _, p := range t.Postings {
					if acctRE.MatchString(p.Account) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if excludeAcctRE != nil {
				excluded := false
				for _, p := range t.Postings {
					if excludeAcctRE.MatchString(p.Account) {
						excluded = true
						break
					}
				}
				if excluded {
					continue
				}
			}
			if payeeRE != nil && !payeeRE.MatchString(t.Description) {
				continue
			}
			if excludePayeeRE != nil && excludePayeeRE.MatchString(t.Description) {
				continue
			}
			if hasAmt {
				if isRange {
					found := false
					for _, p := range t.Postings {
						if !p.Null && p.Value >= minVal && p.Value <= maxVal {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				} else {
					found := false
					for _, p := range t.Postings {
						if !p.Null && p.Value == exactAmt {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}
			}
			tc := *t.CleanCopy()
			results = append(results, queryResult{
				File:  fe.path,
				Ref:   fmt.Sprintf("%d:%s", i, makeRef(fe.path, i, &tc)),
				Entry: &tc,
			})
		}
	}
	return results, nil
}

func makeRef(path string, index int, t *ledger.Transaction) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%d:", path, index)
	fmt.Fprintf(h, "%s:", t.Date.Format("2006/01/02"))
	if !t.ClearDate.IsZero() {
		fmt.Fprintf(h, "%s:", t.ClearDate.Format("2006/01/02"))
	}
	fmt.Fprintf(h, "%s:", t.Description)
	for _, p := range t.Postings {
		fmt.Fprintf(h, "%s:%d:", p.Account, p.Value)
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:8])
}
