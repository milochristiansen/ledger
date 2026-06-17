// Package tools provides programmatic access to ledger file operations:
// query, edit by ref, and format. It consolidates the logic previously spread
// across the queryledger, editledger, and fmtledger CLI tools.
package tools

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse"
)

// QueryParams holds all filter criteria for transaction lookup.
// Zero-value fields are treated as "not filtered."
type QueryParams struct {
	Date           string // "YYYY/MM/DD" or "YYYY/MM/DD:YYYY/MM/DD"
	Account        string // regex
	ExcludeAccount string // regex
	Payee          string // regex
	ExcludePayee   string // regex
	Amount         string // exact ("20.00") or range ("-500.00:0.00"), optional $
	Status         string // "clear"|"*"|"pending"|"!"|"none"
	ExcludeStatus  string
	Tag            string // filter transactions that have this tag
	KV             string // filter by key ("Key") or key+value ("Key:Value")
}

// FilterNode is a node in a filter tree.
// A leaf has Field+Match+Arg1+Arg2, plus optional Invert.
// Next is a list of continuation nodes — at least one must match in full
// for the filter to pass. Zero next items = done.
// AND is built by chaining (each node has exactly one next item).
// OR is built by listing multiple next items.
// An all-empty node matches everything.
type FilterNode struct {
	Field  string       `json:"field,omitempty"`
	Match  string       `json:"match,omitempty"` // "regex"|"exact"|"range"|"has"
	Arg1   string       `json:"arg1,omitempty"`
	Arg2   string       `json:"arg2,omitempty"` // second operand for range/kv-exact
	Invert bool         `json:"invert,omitempty"`
	Next   []FilterNode `json:"next,omitempty"` // at least one must match; 0 = done
}

// QueryResult holds a single matching transaction.
type QueryResult struct {
	File  string              `json:"file"`
	Ref   string              `json:"ref"`
	Entry *ledger.Transaction `json:"entry"`
}

// TagOp describes a tag add/remove operation.
type TagOp struct {
	Op   string `json:"op"` // "add" or "remove"
	Name string `json:"name"`
}

// KVOp describes a key-value pair set/delete operation.
type KVOp struct {
	Op    string `json:"op"` // "set" or "delete"
	Key   string `json:"key"`
	Value string `json:"value,omitempty"` // only for "set"
}

// PostingOp describes an operation on a posting.
// Op is "set" (modify existing), "delete" (remove), or "insert" (add new).
type PostingOp struct {
	Op    string `json:"op"`
	Index int    `json:"index"`
	// Fields for "set" and "insert" (all optional except Account required for insert):
	Account *string `json:"account,omitempty"`
	Amount  *string `json:"amount,omitempty"` // "$20.00"; empty string = set null
	Note    *string `json:"note,omitempty"`
	Assert  *string `json:"assert,omitempty"` // "$20.00"; empty string = clear assert
	Status  *string `json:"status,omitempty"` // "clear"/"*" or "pending"/"!"
}

// EditSpec describes a complete set of edits to apply to a transaction.
// Pointer fields: nil = no change; non-nil = apply ("" clears the field where meaningful).
type EditSpec struct {
	Description *string     `json:"description,omitempty"`
	Date        *string     `json:"date,omitempty"`       // "2006/01/02" format
	ClearDate   *string     `json:"clear_date,omitempty"` // "2006/01/02" format
	Status      *string     `json:"status,omitempty"`     // "clear", "pending", "none", or ""
	Code        *string     `json:"code,omitempty"`       // "" clears the code
	Comment     *string     `json:"comment,omitempty"`    // nil=no change, ""=clear all comments, "text"=set single comment
	TagOps      []TagOp     `json:"tag_ops,omitempty"`
	KVOps       []KVOp      `json:"kv_ops,omitempty"`
	PostingOps  []PostingOp `json:"posting_ops,omitempty"`
}

// TransactionJSON is the structured JSON representation of a ledger transaction.
type TransactionJSON struct {
	Ref         string            `json:"ref"`
	File        string            `json:"file"`
	Line        int               `json:"line"`
	Date        string            `json:"date"` // "2006/01/02"
	ClearDate   string            `json:"clear_date,omitempty"`
	Status      string            `json:"status,omitempty"` // "clear"|"pending"|"none"
	Code        string            `json:"code,omitempty"`
	Description string            `json:"description"`
	Postings    []PostingJSON     `json:"postings"`
	Comments    []string          `json:"comments,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	KV          map[string]string `json:"kv,omitempty"`
}

// PostingJSON is the structured JSON representation of a posting.
type PostingJSON struct {
	Status  string `json:"status,omitempty"` // "clear"|"pending"|"none"
	Account string `json:"account"`
	Amount  string `json:"amount,omitempty"` // "$123.45" or empty if null
	Null    bool   `json:"null"`
	Assert  string `json:"assert,omitempty"`
	Note    string `json:"note,omitempty"`
}

// FormatResult describes the result of a Format operation.
type FormatResult struct {
	Changed bool     `json:"changed"`
	Backup  string   `json:"backup"` // filename only, not full path
	Files   []string `json:"files"`  // all written file paths
}

// NewTransactionJSON converts a QueryResult to TransactionJSON.
func NewTransactionJSON(r *QueryResult) TransactionJSON {
	t := r.Entry
	line, _ := strconv.Atoi(CSVField("line", r))
	out := TransactionJSON{
		Ref:         r.Ref,
		File:        r.File,
		Line:        line,
		Date:        t.Date.Format("2006/01/02"),
		Description: t.Description,
		Comments:    t.Comments,
		Postings:    make([]PostingJSON, len(t.Postings)),
	}
	if !t.ClearDate.IsZero() {
		out.ClearDate = t.ClearDate.Format("2006/01/02")
	}
	switch t.Status {
	case ledger.StatusClear:
		out.Status = "clear"
	case ledger.StatusPending:
		out.Status = "pending"
	default:
		out.Status = "none"
	}
	if t.Code != "" {
		out.Code = t.Code
	}
	for i, p := range t.Postings {
		pj := PostingJSON{Account: p.Account}
		switch p.Status {
		case ledger.StatusClear:
			pj.Status = "clear"
		case ledger.StatusPending:
			pj.Status = "pending"
		}
		if p.Null {
			pj.Null = true
		} else {
			pj.Amount = ledger.FormatValueNumber(p.Value)
		}
		if p.HasAssert {
			pj.Assert = ledger.FormatValueNumber(p.Assert)
		}
		if p.Note != "" {
			pj.Note = p.Note
		}
		out.Postings[i] = pj
	}
	if len(t.Tags) > 0 {
		out.Tags = make([]string, 0, len(t.Tags))
		for tag := range t.Tags {
			out.Tags = append(out.Tags, tag)
		}
	}
	if len(t.KVPairs) > 0 {
		out.KV = make(map[string]string, len(t.KVPairs))
		for k, v := range t.KVPairs {
			out.KV[k] = v
		}
	}
	return out
}

// EditResult describes the result of an Edit operation.
type EditResult struct {
	Ref         string          `json:"ref"`
	Transaction TransactionJSON `json:"transaction"`
}

// lfEntry is an internal helper pairing a parsed file with its path.
type lfEntry struct {
	path string
	f    *ledger.File
}

// Query searches the ledger tree rooted at rootPath for transactions
// matching params. All filters compose with AND semantics.
// Returns results in file order.
func Query(rootPath string, params QueryParams) ([]QueryResult, error) {
	w, err := NewFileSafeWriter(rootPath)
	if err != nil {
		return nil, err
	}
	pis, err := w.Includes(w.Add)
	if err != nil {
		return nil, fmt.Errorf("loading includes: %w", err)
	}
	files := []lfEntry{{filepath.Base(rootPath), w.File}}
	for _, pi := range pis {
		if pi.File != nil {
			files = append(files, lfEntry{pi.Path, pi.File})
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
		statusVal      int
		hasStatus      bool
		excludeStVal   int
		hasExcludeSt   bool
	)

	if params.Date != "" {
		afterD, beforeD, err = parseDateFlag(params.Date)
		if err != nil {
			return nil, err
		}
		hasDate = true
	}
	if params.Account != "" {
		acctRE, err = regexp.Compile(params.Account)
		if err != nil {
			return nil, fmt.Errorf("account regex: %w", err)
		}
	}
	if params.ExcludeAccount != "" {
		excludeAcctRE, err = regexp.Compile(params.ExcludeAccount)
		if err != nil {
			return nil, fmt.Errorf("exclude-account regex: %w", err)
		}
	}
	if params.Payee != "" {
		payeeRE, err = regexp.Compile(params.Payee)
		if err != nil {
			return nil, fmt.Errorf("payee regex: %w", err)
		}
	}
	if params.ExcludePayee != "" {
		excludePayeeRE, err = regexp.Compile(params.ExcludePayee)
		if err != nil {
			return nil, fmt.Errorf("exclude-payee regex: %w", err)
		}
	}
	if params.Status != "" {
		statusVal, err = parseStatusFlag(params.Status)
		if err != nil {
			return nil, fmt.Errorf("status: %w", err)
		}
		hasStatus = true
	}
	if params.ExcludeStatus != "" {
		excludeStVal, err = parseStatusFlag(params.ExcludeStatus)
		if err != nil {
			return nil, fmt.Errorf("exclude-status: %w", err)
		}
		hasExcludeSt = true
	}

	var tagFilter string
	hasTag := params.Tag != ""
	if hasTag {
		tagFilter = params.Tag
	}
	var kvKey, kvValue string
	hasKV := params.KV != ""
	if hasKV {
		if idx := strings.Index(params.KV, ":"); idx >= 0 {
			kvKey = strings.TrimSpace(params.KV[:idx])
			kvValue = strings.TrimSpace(params.KV[idx+1:])
		} else {
			kvKey = params.KV
		}
	}

	var exactAmt, minVal, maxVal int64
	hasAmt := params.Amount != ""
	isRange := hasAmt && strings.Contains(params.Amount, ":")
	if hasAmt {
		if isRange {
			parts := strings.SplitN(params.Amount, ":", 2)
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
			exactAmt, err = parseAmountFlag(params.Amount)
			if err != nil {
				return nil, fmt.Errorf("amount: %w", err)
			}
		}
	}

	var results []QueryResult
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
			if hasStatus && int(t.Status) != statusVal {
				continue
			}
			if hasExcludeSt && int(t.Status) == excludeStVal {
				continue
			}
			if hasTag && !t.Tags[tagFilter] {
				continue
			}
			if hasKV {
				v, ok := t.KVPairs[kvKey]
				if !ok || (kvValue != "" && v != kvValue) {
					continue
				}
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
			results = append(results, QueryResult{
				File:  fe.path,
				Ref:   fmt.Sprintf("%d:%s", i, makeRef(fe.path, i, &tc)),
				Entry: &tc,
			})
		}
	}
	return results, nil
}

// QueryWithFilter searches the ledger tree rooted at rootPath for transactions
// matching the filter tree. A zero-value FilterNode matches all transactions.
func QueryWithFilter(rootPath string, filter FilterNode) ([]QueryResult, error) {
	w, err := NewFileSafeWriter(rootPath)
	if err != nil {
		return nil, err
	}
	pis, err := w.Includes(w.Add)
	if err != nil {
		return nil, fmt.Errorf("loading includes: %w", err)
	}
	files := []lfEntry{{filepath.Base(rootPath), w.File}}
	for _, pi := range pis {
		if pi.File != nil {
			files = append(files, lfEntry{pi.Path, pi.File})
		}
	}

	var results []QueryResult
	for _, fe := range files {
		for i, ent := range fe.f.Entries {
			t, ok := ent.(*ledger.Transaction)
			if !ok {
				continue
			}
			ok, err := matchFilter(t, filter)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			tc := *t.CleanCopy()
			results = append(results, QueryResult{
				File:  fe.path,
				Ref:   fmt.Sprintf("%d:%s", i, makeRef(fe.path, i, &tc)),
				Entry: &tc,
			})
		}
	}
	return results, nil
}

// matchFilter evaluates a filter tree against a transaction.
func matchFilter(t *ledger.Transaction, fn FilterNode) (bool, error) {
	// Match the current leaf (if any).
	if fn.Field != "" {
		result, err := matchLeaf(t, fn.Field, fn.Match, fn.Arg1, fn.Arg2)
		if err != nil {
			return false, err
		}
		if fn.Invert {
			result = !result
		}
		if !result {
			return false, nil
		}
	}
	// At least one next node must match in full.
	for _, next := range fn.Next {
		ok, err := matchFilter(t, next)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	// No next nodes = done; if we had next nodes but none matched, fail.
	return len(fn.Next) == 0, nil
}

// matchLeaf evaluates a single filter leaf against a transaction.
func matchLeaf(t *ledger.Transaction, field, match, arg1, arg2 string) (bool, error) {
	switch field {
	case "date":
		return matchDate(t, match, arg1, arg2)
	case "account":
		return matchAccount(t, match, arg1)
	case "payee":
		return matchPayee(t, match, arg1)
	case "amount":
		return matchAmount(t, match, arg1, arg2)
	case "status":
		return matchStatus(t, match, arg1)
	case "tag":
		return matchTag(t, match, arg1)
	case "kv":
		return matchKV(t, match, arg1, arg2)
	default:
		return false, fmt.Errorf("unknown field %q", field)
	}
}

func matchDate(t *ledger.Transaction, match, arg1, arg2 string) (bool, error) {
	switch match {
	case "exact":
		after, before, err := parseDateFlag(arg1)
		if err != nil {
			return false, fmt.Errorf("invalid date: %s", arg1)
		}
		return !t.Date.Before(after) && t.Date.Before(before), nil
	case "range":
		if arg1 == "" || arg2 == "" {
			return false, fmt.Errorf("date range requires arg1 and arg2")
		}
		after, before, err := parseDateFlag(arg1 + ":" + arg2)
		if err != nil {
			return false, fmt.Errorf("invalid date range: %s:%s", arg1, arg2)
		}
		return !t.Date.Before(after) && t.Date.Before(before), nil
	default:
		return false, fmt.Errorf("unknown match %q for field date", match)
	}
}

func matchAccount(t *ledger.Transaction, match, arg1 string) (bool, error) {
	switch match {
	case "regex":
		re, err := regexp.Compile(arg1)
		if err != nil {
			return false, fmt.Errorf("invalid regex: %s", arg1)
		}
		for _, p := range t.Postings {
			if re.MatchString(p.Account) {
				return true, nil
			}
		}
		return false, nil
	case "exact":
		for _, p := range t.Postings {
			if p.Account == arg1 {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown match %q for field account", match)
	}
}

func matchPayee(t *ledger.Transaction, match, arg1 string) (bool, error) {
	switch match {
	case "regex":
		re, err := regexp.Compile(arg1)
		if err != nil {
			return false, fmt.Errorf("invalid regex: %s", arg1)
		}
		return re.MatchString(t.Description), nil
	case "exact":
		return t.Description == arg1, nil
	default:
		return false, fmt.Errorf("unknown match %q for field payee", match)
	}
}

func matchAmount(t *ledger.Transaction, match, arg1, arg2 string) (bool, error) {
	switch match {
	case "exact":
		exactAmt, err := parseAmountFlag(arg1)
		if err != nil {
			return false, fmt.Errorf("invalid amount: %s", arg1)
		}
		for _, p := range t.Postings {
			if !p.Null && p.Value == exactAmt {
				return true, nil
			}
		}
		return false, nil
	case "range":
		minVal, err := parseAmountFlag(arg1)
		if err != nil {
			return false, fmt.Errorf("invalid amount min: %s", arg1)
		}
		maxVal, err := parseAmountFlag(arg2)
		if err != nil {
			return false, fmt.Errorf("invalid amount max: %s", arg2)
		}
		if minVal > maxVal {
			minVal, maxVal = maxVal, minVal
		}
		for _, p := range t.Postings {
			if !p.Null && p.Value >= minVal && p.Value <= maxVal {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown match %q for field amount", match)
	}
}

func matchStatus(t *ledger.Transaction, match, arg1 string) (bool, error) {
	if match != "exact" {
		return false, fmt.Errorf("unknown match %q for field status", match)
	}
	statusVal, err := parseStatusFlag(arg1)
	if err != nil {
		return false, fmt.Errorf("invalid status: %s", arg1)
	}
	return int(t.Status) == statusVal, nil
}

func matchTag(t *ledger.Transaction, match, arg1 string) (bool, error) {
	if match != "has" {
		return false, fmt.Errorf("unknown match %q for field tag", match)
	}
	return t.Tags[arg1], nil
}

func matchKV(t *ledger.Transaction, match, arg1, arg2 string) (bool, error) {
	switch match {
	case "has":
		_, ok := t.KVPairs[arg1]
		return ok, nil
	case "exact":
		v, ok := t.KVPairs[arg1]
		return ok && v == arg2, nil
	default:
		return false, fmt.Errorf("unknown match %q for field kv", match)
	}
}

// QueryByRef finds a single transaction by its "N:hash" ref handle.
// scopeFile limits the search to one file; empty scans all included files.
func QueryByRef(rootPath, ref, scopeFile string) (*QueryResult, error) {
	w, err := NewFileSafeWriter(rootPath)
	if err != nil {
		return nil, err
	}
	pis, err := w.Includes(w.Add)
	if err != nil {
		return nil, fmt.Errorf("loading includes: %w", err)
	}
	files := []lfEntry{{filepath.Base(rootPath), w.File}}
	for _, pi := range pis {
		if pi.File != nil {
			files = append(files, lfEntry{pi.Path, pi.File})
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

// Edit modifies a transaction identified by ref, applying the given EditSpec.
// Returns the new ref after all edits (ref changes on content change).
// Creates a backup tar.gz before writing if any file changed.
func Edit(rootPath, ref, scopeFile string, spec EditSpec) (string, error) {
	w, err := NewFileSafeWriter(rootPath)
	if err != nil {
		return "", err
	}
	pis, err := w.Includes(w.Add)
	if err != nil {
		return "", err
	}
	files := []lfEntry{{filepath.Base(rootPath), w.File}}
	for _, pi := range pis {
		if pi.File != nil {
			files = append(files, lfEntry{pi.Path, pi.File})
		}
	}

	colon := strings.LastIndex(ref, ":")
	if colon < 0 {
		return "", fmt.Errorf("invalid ref: %s", ref)
	}
	index, err := strconv.Atoi(ref[:colon])
	if err != nil {
		return "", fmt.Errorf("invalid ref index: %s", ref)
	}
	hash := ref[colon+1:]

	var target *ledger.Transaction
	var targetPath string
	if scopeFile != "" {
		for _, fe := range files {
			if fe.path != scopeFile {
				continue
			}
			if t, ok := findInFile(fe, index, hash); ok {
				target, targetPath = t, fe.path
				goto found
			}
			break
		}
	} else {
		for _, fe := range files {
			if t, ok := findInFile(fe, index, hash); ok {
				target, targetPath = t, fe.path
				goto found
			}
		}
	}
	return "", fmt.Errorf("transaction not found for ref: %s", ref)

found:
	if err := applyEdit(target, spec); err != nil {
		return "", err
	}

	if err := w.Commit(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s", index, makeRef(targetPath, index, target)), nil
}

// Format standardizes formatting of all ledger files in the tree rooted
// at rootPath. Creates a backup tar.gz if any file changed.
func Format(rootPath string) (FormatResult, error) {
	w, err := NewFileSafeWriter(rootPath)
	if err != nil {
		return FormatResult{}, err
	}

	_, err = w.Includes(w.Add)
	if err != nil {
		return FormatResult{}, fmt.Errorf("loading includes: %w", err)
	}

	if err := w.Commit(); err != nil {
		return FormatResult{}, err
	}
	return FormatResult{
		Changed: w.Changed,
		Backup:  w.Backup,
		Files:   w.Files,
	}, nil
}

// CSVField extracts a named field from a QueryResult as a string.
// Fields: "file", "line", "ref", "date", "clear_date", "status",
//
//	"code", "description", "account[:N]", "amount[:N]", "note[:N]",
//	"status[:N]", "assert[:N]", "comment", "tag:NAME", "kv:KEY"
func CSVField(name string, r *QueryResult) string {
	t := r.Entry
	file := r.File
	ref := r.Ref
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
		if rest, ok := strings.CutPrefix(name, "account:"); ok {
			ix, _ = strconv.Atoi(rest)
		}
		if ix >= 0 && ix < len(t.Postings) {
			return t.Postings[ix].Account
		}
		return ""
	case strings.HasPrefix(name, "amount"):
		ix := 0
		if rest, ok := strings.CutPrefix(name, "amount:"); ok {
			ix, _ = strconv.Atoi(rest)
		}
		if ix >= 0 && ix < len(t.Postings) && !t.Postings[ix].Null {
			return ledger.FormatValueNumber(t.Postings[ix].Value)
		}
		return ""
	case strings.HasPrefix(name, "note"):
		ix := 0
		if rest, ok := strings.CutPrefix(name, "note:"); ok {
			ix, _ = strconv.Atoi(rest)
		}
		if ix >= 0 && ix < len(t.Postings) {
			return t.Postings[ix].Note
		}
		return ""
	case strings.HasPrefix(name, "status"):
		ix := 0
		if rest, ok := strings.CutPrefix(name, "status:"); ok {
			ix, _ = strconv.Atoi(rest)
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
		if rest, ok := strings.CutPrefix(name, "assert:"); ok {
			ix, _ = strconv.Atoi(rest)
		}
		if ix >= 0 && ix < len(t.Postings) && t.Postings[ix].HasAssert {
			return ledger.FormatValueNumber(t.Postings[ix].Assert)
		}
		return ""
	case name == "comment":
		if len(t.Comments) > 0 {
			return t.Comments[0]
		}
		return ""
	case strings.HasPrefix(name, "tag:"):
		tag, _ := strings.CutPrefix(name, "tag:")
		if t.Tags[tag] {
			return "true"
		}
		return "false"
	case strings.HasPrefix(name, "kv:"):
		key, _ := strings.CutPrefix(name, "kv:")
		return t.KVPairs[key]
	}
	return ""
}

// --- helper functions ---

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

func parseStatusFlag(s string) (int, error) {
	switch strings.ToLower(s) {
	case "clear", "*":
		return int(ledger.StatusClear), nil
	case "pending", "!":
		return int(ledger.StatusPending), nil
	case "none", "":
		return int(ledger.StatusUndefined), nil
	default:
		return 0, fmt.Errorf("unknown status %q (use clear/*, pending/!, or none)", s)
	}
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

func searchFile(fe lfEntry, index int, hash string) *QueryResult {
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
				return &QueryResult{
					File:  fe.path,
					Ref:   fmt.Sprintf("%d:%s", offset, hash),
					Entry: &tc,
				}
			}
		}
	}
	return nil
}

func findInFile(fe lfEntry, index int, hash string) (*ledger.Transaction, bool) {
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
			if makeRef(fe.path, offset, t) == hash {
				return t, true
			}
		}
	}
	return nil, false
}

func applyEdit(t *ledger.Transaction, spec EditSpec) error {
	if spec.Description != nil {
		t.Description = *spec.Description
	}
	if spec.Date != nil {
		d, err := time.Parse("2006/01/02", *spec.Date)
		if err != nil {
			return err
		}
		t.Date = d
	}
	if spec.ClearDate != nil {
		d, err := time.Parse("2006/01/02", *spec.ClearDate)
		if err != nil {
			return err
		}
		t.ClearDate = d
	}
	if spec.Status != nil {
		switch *spec.Status {
		case "clear", "*":
			t.Status = ledger.StatusClear
		case "pending", "!":
			t.Status = ledger.StatusPending
		case "none", "":
			t.Status = 0
		default:
			return fmt.Errorf("invalid status: %s", *spec.Status)
		}
	}
	if spec.Code != nil {
		t.Code = *spec.Code
	}
	if spec.Comment != nil {
		if *spec.Comment == "" {
			t.Comments = nil
		} else {
			t.Comments = []string{*spec.Comment}
		}
	}

	for _, to := range spec.TagOps {
		switch to.Op {
		case "add":
			if t.Tags == nil {
				t.Tags = map[string]bool{}
			}
			t.Tags[to.Name] = true
		case "remove":
			delete(t.Tags, to.Name)
		default:
			return fmt.Errorf("unknown tag op: %s", to.Op)
		}
	}

	for _, ko := range spec.KVOps {
		switch ko.Op {
		case "set":
			if t.KVPairs == nil {
				t.KVPairs = map[string]string{}
			}
			t.KVPairs[ko.Key] = ko.Value
		case "delete":
			delete(t.KVPairs, ko.Key)
		default:
			return fmt.Errorf("unknown kv op: %s", ko.Op)
		}
	}

	for _, po := range spec.PostingOps {
		switch po.Op {
		case "set":
			if po.Index < 0 || po.Index >= len(t.Postings) {
				return fmt.Errorf("posting index out of range: %d", po.Index)
			}
			p := &t.Postings[po.Index]
			if po.Account != nil {
				p.Account = *po.Account
			}
			if po.Amount != nil {
				if *po.Amount == "" {
					p.Value = 0
					p.Null = true
				} else {
					v, null, err := parse.ReadAmount(parse.NewCharReader(*po.Amount+"\n", 1))
					if err != nil {
						return err
					}
					if null {
						return fmt.Errorf("amount must not be empty")
					}
					p.Value = v
					p.Null = false
				}
			}
			if po.Note != nil {
				p.Note = *po.Note
			}
			if po.Assert != nil {
				if *po.Assert == "" {
					p.HasAssert = false
				} else {
					v, null, err := parse.ReadAmount(parse.NewCharReader(*po.Assert+"\n", 1))
					if err != nil {
						return err
					}
					if null {
						return fmt.Errorf("assert amount must not be empty")
					}
					p.Assert = v
					p.HasAssert = true
				}
			}
			if po.Status != nil {
				switch *po.Status {
				case "clear", "*":
					p.Status = ledger.StatusClear
				case "pending", "!":
					p.Status = ledger.StatusPending
				case "none", "":
					p.Status = 0
				default:
					return fmt.Errorf("invalid posting status: %s", *po.Status)
				}
			}
		case "delete":
			if po.Index < 0 || po.Index >= len(t.Postings) {
				return fmt.Errorf("posting index out of range: %d", po.Index)
			}
			t.Postings = append(t.Postings[:po.Index], t.Postings[po.Index+1:]...)
		case "insert":
			if po.Index < 0 || po.Index > len(t.Postings) {
				return fmt.Errorf("posting index out of range: %d", po.Index)
			}
			if po.Account == nil {
				return fmt.Errorf("account required for insert")
			}
			p := ledger.Posting{Account: *po.Account, Null: true}
			if po.Amount != nil {
				v, null, err := parse.ReadAmount(parse.NewCharReader(*po.Amount+"\n", 1))
				if err != nil {
					return err
				}
				if !null {
					p.Value = v
					p.Null = false
				}
			}
			if po.Note != nil {
				p.Note = *po.Note
			}
			if po.Assert != nil {
				v, null, err := parse.ReadAmount(parse.NewCharReader(*po.Assert+"\n", 1))
				if err != nil {
					return err
				}
				if !null {
					p.Assert = v
					p.HasAssert = true
				}
			}
			if po.Status != nil {
				switch *po.Status {
				case "clear", "*":
					p.Status = ledger.StatusClear
				case "pending", "!":
					p.Status = ledger.StatusPending
				}
			}
			if po.Index == len(t.Postings) {
				t.Postings = append(t.Postings, p)
			} else {
				t.Postings = append(t.Postings, ledger.Posting{})
				copy(t.Postings[po.Index+1:], t.Postings[po.Index:])
				t.Postings[po.Index] = p
			}
		default:
			return fmt.Errorf("unknown posting op: %s", po.Op)
		}
	}
	return nil
}
