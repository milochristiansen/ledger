package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse"
	"github.com/milochristiansen/ledger/tools"
)

var (
	sets     flagValue
	refFlag  = flag.String("ref", "", "ref code of the transaction to edit (required)")
	fileFlag = flag.String("file", "", "limit search to this file")
)

type flagValue []struct{ key, value string }

func (f *flagValue) String() string { return fmt.Sprint(*f) }
func (f *flagValue) Set(s string) error {
	k, v, ok := strings.Cut(s, "=")
	if !ok {
		return fmt.Errorf("expected Field=Value, got %q", s)
	}
	*f = append(*f, struct{ key, value string }{strings.TrimSpace(k), strings.TrimSpace(v)})
	return nil
}

func init() { flag.Var(&sets, "set", "edit transaction fields (see usage)") }

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: editledger -ref <ref> [-file <file>] [-set Field=Value ...] <ledger-file>\n"+
			"\ntransaction:\n"+
			"  description, date, clear_date, status (clear/pending/none), code\n"+
			"  account, account:N   (posting account)\n"+
			"  amount, amount:N     (posting amount, empty clears to null)\n"+
			"  assert, assert:N     (balance assertion, empty clears)\n"+
			"  note, note:N         (posting note)\n"+
			"  posting:N=AccountName    (empty value deletes posting at N)\n"+
			"\nflags:\n")
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *refFlag == "" {
		fmt.Fprintf(os.Stderr, "error: -ref is required\n")
		os.Exit(2)
	}
	if len(sets) == 0 {
		fmt.Fprintf(os.Stderr, "error: at least one -set flag required\n")
		os.Exit(2)
	}
	newRef, err := edit(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(newRef)
}

type fileEntry struct {
	path string
	f    *ledger.File
}

func edit(rootPath string) (string, error) {
	w, err := tools.NewFileSafeWriter(rootPath)
	if err != nil {
		return "", err
	}
	pis, err := w.Includes(w.Add)
	if err != nil {
		return "", err
	}
	files := []fileEntry{{filepath.Base(rootPath), w.File}}
	for _, pi := range pis {
		if pi.File != nil {
			files = append(files, fileEntry{pi.Path, pi.File})
		}
	}

	colon := strings.LastIndex(*refFlag, ":")
	if colon < 0 {
		return "", fmt.Errorf("invalid ref: %s", *refFlag)
	}
	index, err := strconv.Atoi((*refFlag)[:colon])
	if err != nil {
		return "", fmt.Errorf("invalid ref index: %s", *refFlag)
	}
	hash := (*refFlag)[colon+1:]

	var target *ledger.Transaction
	var targetPath string
	if *fileFlag != "" {
		for _, fe := range files {
			if fe.path != *fileFlag {
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
	return "", fmt.Errorf("transaction not found for ref: %s", *refFlag)

found:
	for _, s := range sets {
		if err := applyEdit(target, s.key, s.value); err != nil {
			return "", fmt.Errorf("%s: %w", s.key, err)
		}
	}

	if err := w.Commit(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s", index, makeRef(targetPath, index, target)), nil
}

func applyEdit(t *ledger.Transaction, field, value string) error {

	if parts := strings.SplitN(field, ":", 2); len(parts) == 2 && parts[0] == "posting" {
		return insertOrDeletePosting(t, parts[1], value)
	}

	switch {
	case field == "description":
		t.Description = value
	case field == "date":
		d, err := time.Parse("2006/01/02", value)
		if err != nil {
			return err
		}
		t.Date = d
	case field == "clear_date":
		d, err := time.Parse("2006/01/02", value)
		if err != nil {
			return err
		}
		t.ClearDate = d
	case field == "status":
		return setTxStatus(t, value)
	case field == "code":
		t.Code = value
	case field == "account":
		if len(t.Postings) == 0 {
			return fmt.Errorf("no postings")
		}
		t.Postings[0].Account = value
	case strings.HasPrefix(field, "account:"):
		ix, err := strconv.Atoi(strings.TrimPrefix(field, "account:"))
		if err != nil || ix < 0 || ix >= len(t.Postings) {
			return fmt.Errorf("invalid posting index: %s", field)
		}
		t.Postings[ix].Account = value
	case field == "amount":
		if value == "" {
			if len(t.Postings) > 0 {
				t.Postings[0].Value = 0
				t.Postings[0].Null = true
			}
			return nil
		}
		return setPostingAmount(t, 0, value)
	case strings.HasPrefix(field, "amount:"):
		ix, err := strconv.Atoi(strings.TrimPrefix(field, "amount:"))
		if err != nil || ix < 0 || ix >= len(t.Postings) {
			return fmt.Errorf("invalid posting index: %s", field)
		}
		if value == "" {
			t.Postings[ix].Value = 0
			t.Postings[ix].Null = true
			return nil
		}
		return setPostingAmount(t, ix, value)
	case field == "assert":
		if len(t.Postings) > 0 {
			return setPostingAssert(t, 0, value)
		}
		return nil
	case strings.HasPrefix(field, "assert:"):
		ix, err := strconv.Atoi(strings.TrimPrefix(field, "assert:"))
		if err != nil || ix < 0 || ix >= len(t.Postings) {
			return fmt.Errorf("invalid posting index: %s", field)
		}
		return setPostingAssert(t, ix, value)
	case field == "note":
		if len(t.Postings) > 0 {
			t.Postings[0].Note = value
		}
		return nil
	case strings.HasPrefix(field, "note:"):
		ix, err := strconv.Atoi(strings.TrimPrefix(field, "note:"))
		if err != nil || ix < 0 || ix >= len(t.Postings) {
			return fmt.Errorf("invalid posting index: %s", field)
		}
		t.Postings[ix].Note = value
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
	return nil
}

func insertOrDeletePosting(t *ledger.Transaction, ixStr, account string) error {
	ix, err := strconv.Atoi(ixStr)
	if err != nil {
		return fmt.Errorf("invalid posting index: %s", ixStr)
	}
	if account == "" {
		// delete: must be a valid existing index
		if ix < 0 || ix >= len(t.Postings) {
			return fmt.Errorf("index out of range: %d", ix)
		}
		t.Postings = append(t.Postings[:ix], t.Postings[ix+1:]...)
		return nil
	}
	// insert: allow ix == len(t.Postings) to append
	if ix < 0 || ix > len(t.Postings) {
		return fmt.Errorf("index out of range: %d", ix)
	}
	p := ledger.Posting{Account: account, Null: true}
	if ix == len(t.Postings) {
		t.Postings = append(t.Postings, p)
		return nil
	}
	t.Postings = append(t.Postings, ledger.Posting{})
	copy(t.Postings[ix+1:], t.Postings[ix:])
	t.Postings[ix] = p
	return nil
}

func setTxStatus(t *ledger.Transaction, value string) error {
	switch value {
	case "clear", "*":
		t.Status = ledger.StatusClear
	case "pending", "!":
		t.Status = ledger.StatusPending
	case "none", "":
		t.Status = 0
	default:
		return fmt.Errorf("invalid status: %s", value)
	}
	return nil
}

func setPostingAmount(t *ledger.Transaction, ix int, s string) error {
	v, null, err := parse.ReadAmount(parse.NewCharReader(s+"\n", 1))
	if err != nil {
		return err
	}
	if null {
		return fmt.Errorf("amount must not be empty")
	}
	t.Postings[ix].Value = v
	t.Postings[ix].Null = false
	return nil
}

func setPostingAssert(t *ledger.Transaction, ix int, s string) error {
	v, null, err := parse.ReadAmount(parse.NewCharReader(s+"\n", 1))
	if err != nil {
		return err
	}
	if null {
		t.Postings[ix].HasAssert = false
		return nil
	}
	t.Postings[ix].Assert = v
	t.Postings[ix].HasAssert = true
	return nil
}

func findInFile(fe fileEntry, index int, hash string) (*ledger.Transaction, bool) {
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
