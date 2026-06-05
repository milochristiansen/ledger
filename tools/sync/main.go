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

2. Altered source versions must be plainly marked as such, and must not be
misrepresented as being the original software.

3. This notice may not be removed or altered from any source distribution.
*/

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/milochristiansen/ledger"
	"github.com/milochristiansen/ledger/parse"
	"github.com/milochristiansen/ledger/tools"
)

func main() {
	fs := tools.CommonFlagSet(tools.FlagDestFile|tools.FlagSourceFile|tools.FlagID|tools.FlagRID, usage)
	server := false
	fs.Flags.BoolVar(&server, "server", server, "Act as a server and listen for incoming connections.")
	addr := "http://localhost:2443"
	fs.Flags.StringVar(&addr, "addr", addr, "Address to connect or listen to.")
	fs.Parse()

	// Read master file and setup internal state.
	mf := tools.LoadLedgerFile(fs.MasterFile)

	if !server {
		// Tail the file
		tf := tools.LTail(mf, fs.ID, fs.RID)

		body := new(bytes.Buffer)
		tools.HandleErr(tf.Format(body))

		// Open connection to the server and send the tailed file through.
		r := tools.HandleErrV(http.Post(addr, "text/x-ledger-cli", body))
		tools.HandleErrS(r.StatusCode != http.StatusOK, "Response from server not OK: "+r.Status)

		// Receive result
		sf, err := parse.ParseLedger(parse.NewRawCharReader(bufio.NewReader(r.Body), 1))
		r.Body.Close()
		tools.HandleErr(err)

		// Zipper our data with their data.
		rf := tools.Zipper(tf, sf)

		// Write the result out.
		tools.WriteLedgerFile(fs.DestFile, rf)
		return
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Read incoming transactions
		cf, err := parse.ParseLedger(parse.NewRawCharReader(bufio.NewReader(r.Body), 1))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Find the ID/RID of the first transaction
		tf := &ledger.File{}
		ct := cf.Transactions()
		if len(ct) > 0 {
			cid, ok := ct[0].KVPairs["ID"]
			if !ok {
				fmt.Fprintln(os.Stderr, "Missing ID on first transaction of sent data.")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			crid, ok := ct[0].KVPairs["RID"]
			if !ok {
				fmt.Fprintln(os.Stderr, "Missing RID on first transaction of sent data.")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Tail our file with this information
			tf = tools.LTail(mf, cid, crid)
		}

		// Zipper their data with our data (do it now so we can send back an error if needed).
		xf, err := tools.ZipperHTTP(mf, cf)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Store our new file
		f, err := os.Create(fs.DestFile + "/" + time.Now().UTC().Format("m01-d02-t150405.00") + ".ledger")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer f.Close()
		xf.Format(f)

		// Send back the tailed data from earlier (or an error)
		err = tf.Format(w)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
	})

	tools.HandleErr(http.ListenAndServe(addr, nil))
}

var usage = `Usage:

This program takes a ledger file and either listens for a client to connect and
send an update, or sends an update to a remote copy of this program that is
listening.

For this to work properly, each transaction needs an "ID" K/V to be set to a
unique transaction ID, otherwise it is not possible to sync partial files
and syncing full files is not deterministic. Any non-deterministic result is
an error.

The "master" file is used to set the initial state of the program.

"dest" should be a directory used to write the result of each received sync
when in "server" mode, or the path to the output file for the normal send mode.

For "server" mode the address is the ip:port to listen on.
`
