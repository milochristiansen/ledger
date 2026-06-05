package tools

import (
	"time"

	"github.com/teris-io/shortid"
)

// IDService is a read-only channel that returns a short ID string on every read.
var IDService <-chan string

func init() {
	c := make(chan string)
	IDService = c

	go func() {
		idsource := shortid.MustNew(1, shortid.DefaultABC, uint64(time.Now().UnixNano()))

		for {
			c <- idsource.MustGenerate()
		}
	}()
}
