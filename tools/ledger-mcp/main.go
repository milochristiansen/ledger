package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/milochristiansen/ledger/tools"
)

func main() {
	s := server.NewMCPServer(
		"ledger",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// --- ledger_query ---
	queryTool := mcp.NewTool("ledger_query",
		mcp.WithDescription("Search ledger transactions using a composable filter tree. Returns structured JSON with ref, file, line, date, status, description, and postings (account, amount, note, assert, status). A zero-value or omitted filter matches all transactions."),
		mcp.WithString("root_path", mcp.Required(), mcp.Description("Path to the root ledger file")),
		mcp.WithString("ref", mcp.Description("Find by \"N:hash\" ref; if set, all filter params ignored")),
		mcp.WithString("scope_file", mcp.Description("Limit ref search to this file")),
		mcp.WithAny("filter",
			mcp.Description("Filter tree object. Omit to match all transactions.\n\n"+
				"Leaf fields:\n"+
				"  field  — \"date\" | \"account\" | \"payee\" | \"amount\" | \"status\" | \"tag\" | \"kv\"\n"+
				"  match  — match type (varies by field; see table below)\n"+
				"  arg1   — primary operand (required for all leaves)\n"+
				"  arg2   — secondary operand for range/kv-exact (optional)\n"+
				"  invert — bool, negates the leaf result (default false)\n"+
				"\n"+
				"Match types by field:\n"+
				"  date:    exact (arg1=YYYY/MM/DD) | range (arg1/arg2=YYYY/MM/DD)\n"+
				"  account: regex (arg1=pattern)    | exact (arg1=full account name)\n"+
				"  payee:   regex (arg1=pattern)    | exact (arg1=full description)\n"+
				"  amount:  exact (arg1=12.34)       | range (arg1=min, arg2=max, inclusive)\n"+
				"  status:  exact (arg1=clear|pending|none)\n"+
				"  tag:     has  (arg1=tag name)\n"+
				"  kv:      has  (arg1=key)          | exact (arg1=key, arg2=value)\n"+
				"\n"+
				"Composition:\n"+
				"  AND — chain via \"next\": [singleNode] (exactly one item in next)\n"+
				"  OR  — list multiple alternatives in \"next\": [branchA, branchB, ...]\n"+
				"  Invert — set \"invert\": true on any leaf to negate it\n"+
				"\n"+
				"Examples:\n"+
				"  All Food transactions:\n"+
				"    {\"field\":\"account\",\"match\":\"regex\",\"arg1\":\"Food\"}\n"+
				"  Food AND August 2025:\n"+
				"    {\"field\":\"account\",\"match\":\"regex\",\"arg1\":\"Food\",\"next\":[{\"field\":\"date\",\"match\":\"range\",\"arg1\":\"2025/08/01\",\"arg2\":\"2025/08/31\"}]}\n"+
				"  Food OR Rent (OR via empty root with 2 next items):\n"+
				"    {\"next\":[{\"field\":\"account\",\"match\":\"regex\",\"arg1\":\"Food\"},{\"field\":\"account\",\"match\":\"regex\",\"arg1\":\"Rent\"}]}\n"+
				"  Exclude Checking (invert):\n"+
				"    {\"field\":\"account\",\"match\":\"regex\",\"arg1\":\"Checking\",\"invert\":true}\n"+
				"  Amount between 10 and 50:\n"+
				"    {\"field\":\"amount\",\"match\":\"range\",\"arg1\":\"10.00\",\"arg2\":\"50.00\"}\n"+
				"  Has tag \"vacation\":\n"+
				"    {\"field\":\"tag\",\"match\":\"has\",\"arg1\":\"vacation\"}\n"+
				"  Has key-value Receipt=12345:\n"+
				"    {\"field\":\"kv\",\"match\":\"exact\",\"arg1\":\"Receipt\",\"arg2\":\"12345\"}"),
		),
		mcp.WithOutputSchema[[]tools.TransactionJSON](),
	)
	s.AddTool(queryTool, handleQuery)

	// --- ledger_edit ---
	editTool := mcp.NewTool("ledger_edit",
		mcp.WithDescription("Edit a ledger transaction identified by ref. Creates a backup before modifying."),
		mcp.WithString("root_path", mcp.Required(), mcp.Description("Path to the root ledger file")),
		mcp.WithString("ref", mcp.Required(), mcp.Description("The ref code of the transaction to edit")),
		mcp.WithString("scope_file", mcp.Description("Limit search to this specific file")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("date", mcp.Description("New date: YYYY/MM/DD")),
		mcp.WithString("clear_date", mcp.Description("New clear date: YYYY/MM/DD")),
		mcp.WithString("status", mcp.Description("New status: clear, pending, or none")),
		mcp.WithString("code", mcp.Description("New code (empty clears)")),
		mcp.WithString("comment", mcp.Description("Replace comments; empty clears")),
		mcp.WithArray("tag_ops",
			mcp.Description("Tag add/remove operations"),
			mcp.Items(map[string]any{
				"type":     "object",
				"required": []any{"op", "name"},
				"properties": map[string]any{
					"op":   map[string]any{"type": "string", "enum": []any{"add", "remove"}},
					"name": map[string]any{"type": "string"},
				},
			}),
		),
		mcp.WithArray("kv_ops",
			mcp.Description("Key-value set/delete operations"),
			mcp.Items(map[string]any{
				"type":     "object",
				"required": []any{"op", "key"},
				"properties": map[string]any{
					"op":    map[string]any{"type": "string", "enum": []any{"set", "delete"}},
					"key":   map[string]any{"type": "string"},
					"value": map[string]any{"type": "string"},
				},
			}),
		),
		mcp.WithArray("posting_ops",
			mcp.Description("Posting operations: set, delete, or insert"),
			mcp.Items(map[string]any{
				"type":     "object",
				"required": []any{"op", "index"},
				"properties": map[string]any{
					"op":      map[string]any{"type": "string", "enum": []any{"set", "delete", "insert"}},
					"index":   map[string]any{"type": "integer"},
					"account": map[string]any{"type": "string"},
					"amount":  map[string]any{"type": "string"},
					"note":    map[string]any{"type": "string"},
					"assert":  map[string]any{"type": "string"},
					"status":  map[string]any{"type": "string", "enum": []any{"clear", "*", "pending", "!", "none", ""}},
				},
			}),
		),
		mcp.WithOutputSchema[tools.EditResult](),
	)
	s.AddTool(editTool, handleEdit)

	// --- ledger_format ---
	formatTool := mcp.NewTool("ledger_format",
		mcp.WithDescription("Standardize formatting of all ledger files in the tree. Creates a backup if files changed."),
		mcp.WithString("root_path", mcp.Required(), mcp.Description("Path to the root ledger file")),
		mcp.WithOutputSchema[tools.FormatResult](),
	)
	s.AddTool(formatTool, handleFormat)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
	}
}

func getOptionalString(request mcp.CallToolRequest, key string) string {
	return request.GetString(key, "")
}

func handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rootPath, err := request.RequireString("root_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ref := getOptionalString(request, "ref")
	scopeFile := getOptionalString(request, "scope_file")

	if ref != "" {
		result, err := tools.QueryByRef(rootPath, ref, scopeFile)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if result == nil {
			return mcp.NewToolResultStructuredOnly([]tools.TransactionJSON{}), nil
		}
		tj := tools.NewTransactionJSON(result)
		return mcp.NewToolResultStructuredOnly([]tools.TransactionJSON{tj}), nil
	}

	// Parse the filter param from arguments; zero-value FilterNode matches all.
	var filter tools.FilterNode
	if rawFilter, ok := request.GetArguments()["filter"]; ok && rawFilter != nil {
		b, err := json.Marshal(rawFilter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid filter: %v", err)), nil
		}
		if err := json.Unmarshal(b, &filter); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid filter: %v", err)), nil
		}
	}

	results, err := tools.QueryWithFilter(rootPath, filter)
	if err != nil {

		return mcp.NewToolResultError(err.Error()), nil
	}

	out := make([]tools.TransactionJSON, len(results))
	for i, r := range results {
		out[i] = tools.NewTransactionJSON(&r)
	}
	return mcp.NewToolResultStructuredOnly(out), nil
}

// editArgs merges routing fields with all EditSpec fields.
type editArgs struct {
	RootPath  string `json:"root_path"`
	Ref       string `json:"ref"`
	ScopeFile string `json:"scope_file,omitempty"`
	tools.EditSpec
}

func handleEdit(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args editArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	if args.RootPath == "" {
		return mcp.NewToolResultError("root_path is required"), nil
	}
	if args.Ref == "" {
		return mcp.NewToolResultError("ref is required"), nil
	}

	newRef, err := tools.Edit(args.RootPath, args.Ref, args.ScopeFile, args.EditSpec)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := tools.QueryByRef(args.RootPath, newRef, "")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	tj := tools.NewTransactionJSON(result)
	editResult := tools.EditResult{Ref: newRef, Transaction: tj}
	return mcp.NewToolResultStructuredOnly(editResult), nil
}

func handleFormat(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rootPath, err := request.RequireString("root_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := tools.Format(rootPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultStructuredOnly(result), nil
}
