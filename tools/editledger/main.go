package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

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
			"  comment              (transaction comment, empty clears)\n"+
			"  tag:NAME=true|false   (add or remove a tag)\n"+
			"  kv=Key: Value        (set key-value pair, or kv=Key to delete)\n"+
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
	spec, err := buildEditSpec(sets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	newRef, err := tools.Edit(flag.Arg(0), *refFlag, *fileFlag, spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(newRef)
}

func ptr[T any](v T) *T { return &v }

func buildEditSpec(sets flagValue) (tools.EditSpec, error) {
	var spec tools.EditSpec
	postingAcc := make(map[int]*tools.PostingOp)

	for _, s := range sets {
		key, value := s.key, s.value
		// Handle posting:N operations (insert/delete)
		if strings.HasPrefix(key, "posting:") {
			ix, err := strconv.Atoi(strings.TrimPrefix(key, "posting:"))
			if err != nil {
				return spec, fmt.Errorf("invalid posting index in %q: %w", key, err)
			}
			if value == "" {
				spec.PostingOps = append(spec.PostingOps, tools.PostingOp{Op: "delete", Index: ix})
			} else {
				spec.PostingOps = append(spec.PostingOps, tools.PostingOp{Op: "insert", Index: ix, Account: &value})
			}
			continue
		}

		// Handle fields with optional :N suffix for posting operations
		if parts := strings.SplitN(key, ":", 2); len(parts) == 2 {
			field, idxStr := parts[0], parts[1]
			ix, err := strconv.Atoi(idxStr)
			if err != nil {
				return spec, fmt.Errorf("invalid index in %q: %w", key, err)
			}
			po, ok := postingAcc[ix]
			if !ok {
				po = &tools.PostingOp{Op: "set", Index: ix}
				postingAcc[ix] = po
			}
			switch field {
			case "account":
				po.Account = &value
			case "amount":
				v := value
				po.Amount = &v
			case "assert":
				v := value
				po.Assert = &v
			case "note":
				po.Note = &value
			case "status":
				po.Status = &value
			default:
				return spec, fmt.Errorf("unknown posting field: %s", field)
			}
			continue
		}

		// Transaction-level fields
		switch key {
		case "description":
			spec.Description = &value
		case "date":
			spec.Date = &value
		case "clear_date":
			spec.ClearDate = &value
		case "status":
			spec.Status = &value
		case "code":
			spec.Code = &value
		case "comment":
			spec.Comment = &value
		case "account":
			po, ok := postingAcc[0]
			if !ok {
				po = &tools.PostingOp{Op: "set", Index: 0}
				postingAcc[0] = po
			}
			po.Account = &value
		case "amount":
			v := value
			po, ok := postingAcc[0]
			if !ok {
				po = &tools.PostingOp{Op: "set", Index: 0}
				postingAcc[0] = po
			}
			po.Amount = &v
		case "assert":
			v := value
			po, ok := postingAcc[0]
			if !ok {
				po = &tools.PostingOp{Op: "set", Index: 0}
				postingAcc[0] = po
			}
			po.Assert = &v
		case "note":
			po, ok := postingAcc[0]
			if !ok {
				po = &tools.PostingOp{Op: "set", Index: 0}
				postingAcc[0] = po
			}
			po.Note = &value
		case "kv":
			if idx := strings.Index(value, ":"); idx >= 0 {
				k := strings.TrimSpace(value[:idx])
				v := strings.TrimSpace(value[idx+1:])
				spec.KVOps = append(spec.KVOps, tools.KVOp{Op: "set", Key: k, Value: v})
			} else {
				spec.KVOps = append(spec.KVOps, tools.KVOp{Op: "delete", Key: strings.TrimSpace(value)})
			}
		default:
			if strings.HasPrefix(key, "tag:") {
				name, _ := strings.CutPrefix(key, "tag:")
				if name == "" {
					return spec, fmt.Errorf("tag name required")
				}
				switch value {
				case "true":
					spec.TagOps = append(spec.TagOps, tools.TagOp{Op: "add", Name: name})
				case "false":
					spec.TagOps = append(spec.TagOps, tools.TagOp{Op: "remove", Name: name})
				default:
					return spec, fmt.Errorf("tag value must be true or false")
				}
			} else {
				return spec, fmt.Errorf("unknown field: %s", key)
			}
		}
	}

	// Flatten accumulated posting ops
	for _, po := range postingAcc {
		spec.PostingOps = append(spec.PostingOps, *po)
	}
	return spec, nil
}
