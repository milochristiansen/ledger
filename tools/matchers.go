package tools

import (
	"regexp"

	"github.com/milochristiansen/ledger"
)

// Matcher associates an account or a payee with a regexp to match against a transaction description.
type Matcher struct {
	R       *regexp.Regexp
	Account string
	Payee   string
}

// MatchTransaction replaces the given account in the postings with the first matcher that succeeds.
// If that matcher has a payee, that payee will replace the transaction's description.
// Returns true if any matcher succeeded.
func MatchTransaction(t *ledger.Transaction, account string, matchers []Matcher) bool {
	postingIxs := []int{}
	for i, p := range t.Postings {
		if p.Account == account {
			postingIxs = append(postingIxs, i)
		}
	}

	if len(postingIxs) == 0 {
		return false
	}

	for _, matcher := range matchers {
		if matcher.R.MatchString(t.Description) {
			if matcher.Payee != "" {
				t.Description = matcher.Payee
			}
			for _, ix := range postingIxs {
				t.Postings[ix].Account = matcher.Account
			}
			return true
		}
	}

	return false
}

// MatchedTransactions finds transactions by regexp on the description, and returns a slice
// of found transactions with postings and description modified by the first successful match
// from matchers. Only transactions with a posting containing the given account will be modified.
func MatchedTransactions(f *ledger.File, account string, matchers []Matcher) []ledger.Transaction {
	outTrs := []ledger.Transaction{}
	for _, e := range f.Entries {
		tp, ok := e.(*ledger.Transaction)
		if !ok {
			continue
		}
		cp := *tp.CleanCopy()
		if MatchTransaction(&cp, account, matchers) {
			cp.KVPairs["RID"] = <-IDService
			outTrs = append(outTrs, cp)
		}
	}
	return outTrs
}

// ParseMatchers parses matchers from the directives of a ledger file.
func ParseMatchers(f *ledger.File) ([]Matcher, error) {
	accounts, err := f.Accounts()
	if err != nil {
		return nil, err
	}

	payees, _ := f.Payees()

	matchers := []Matcher{}

	for _, acct := range accounts {
		account := acct.Name

		pm := []Matcher{}
		for _, reStr := range acct.Payees {
			re, err := regexp.Compile(reStr)
			if err != nil {
				return nil, err
			}

			pm = append(pm, Matcher{
				Account: account,
				R:       re,
			})
		}

		for _, payee := range payees {
			for _, m := range pm {
				if m.R.MatchString(payee.Name) {
					for _, alias := range payee.Aliases {
						re, err := regexp.Compile(alias)
						if err != nil {
							return nil, err
						}

						matchers = append(matchers, Matcher{
							Account: account,
							Payee:   payee.Name,
							R:       re,
						})
					}
				}
			}
		}

		matchers = append(matchers, pm...)
	}

	return matchers, nil
}
