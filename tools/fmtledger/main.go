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
	result, err := tools.Format(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if result.Changed {
		fmt.Printf("Formatted %d files; backup: %s\n", len(result.Files), result.Backup)
	} else {
		fmt.Println("Already formatted — no changes.")
	}
}
