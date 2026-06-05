package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/milochristiansen/ledger/tools"
)

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: fmtledger <ledger-file>\n")
		os.Exit(2)
	}
	if err := process(flag.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func process(rootPath string) error {
	w, err := tools.NewFileSafeWriter(rootPath)
	if err != nil {
		return err
	}

	pis, err := w.Includes(w.Add)
	if err != nil {
		return fmt.Errorf("loading includes: %w", err)
	}

	if err := w.Commit(); err != nil {
		return err
	}
	fmt.Printf("formatted %d files\n", len(pis)+1)
	return nil
}
