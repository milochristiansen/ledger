---
name: ledger-tools
description: Query, edit, and format ledger-cli financial ledgers. Use when working with .ledger files for transaction lookup, editing, or pretty-printing.
globs:
  - "*.ledger"
---

# Ledger Tools

Three CLI tools for working with ledger-cli files: `queryledger`, `editledger`, and `fmtledger`.

Install via module path:

```bash
go install github.com/milochristiansen/ledger/tools/queryledger@latest
go install github.com/milochristiansen/ledger/tools/editledger@latest
go install github.com/milochristiansen/ledger/tools/fmtledger@latest
```

## Backups

`editledger` and `fmtledger` create a timestamped backup before writing changes. `queryledger` is read-only and does not.

- Saved in the same directory as the root ledger file.
- Named `backup-YYYYMMDD-HHMMSS.tar.gz`.
- Contains the **original** content of every file in the ledger tree (root + all includes) at the time of the write — even unchanged files.
- No backup is created if no files changed (e.g. `fmtledger` on already-formatted content).

```bash
# Restore from backup
tar -xzf backup-20250824-143052.tar.gz
```

## Query Tool (`queryledger`)

Search transactions by account, amount, date, payee, or ref handle.

```bash
queryledger [flags] <ledger-file>
```

### Query by criteria

| Flag | Syntax | Example |
|---|---|---|
| `-date` | `YYYY/MM/DD` or `YYYY/MM/DD:YYYY/MM/DD` | `-date 2025/08/24` or `-date 2025/08/01:2025/08/31` |
| `-account` | regex | `-account Electronics` |
| `-exclude-account` | regex | `-exclude-account 'PC build'` |
| `-payee` | regex (matches description) | `-payee Amazon` |
| `-exclude-payee` | regex | `-exclude-payee 'Pizza\\|Sub'` |
| `-amount` | exact or range (`$` optional) | `-amount '12.75'` or `-amount '-500.00:0.00'` |
| `-status` | `clear`, `*`, `pending`, `!`, or `none` | `-status clear` or `-status '*'` |
| `-exclude-status` | `clear`, `*`, `pending`, `!`, or `none` | `-exclude-status pending` |

All flags compose with AND semantics.

### Query by ref

```bash
queryledger -ref N:hash [-file 2025.ledger] transactions.ledger
```

The `-file` flag limits ref lookup to a single file (fast). Without it, the tool does a breadth-first scan across all included files (slow for large ledgers).

Ref handles appear in output comments: `; 2025.ledger:1085, ref: 223:c2bba691fd1d8725`

### Output formats

| Flag | Description |
|---|---|
| _(default)_ | Ledger format with `; file:line, ref: N:hash` header comments |
| `-json` | Structured JSON |
| `-csv field,field,...` | Tab-separated values |

**CSV fields:** `date`, `description`, `account`, `account:N`, `amount`, `amount:N`, `note`, `note:N`, `status:N`, `assert`, `assert:N`, `file`, `line`, `ref`, `status`, `code`, `clear_date`

All `:N` indexes are 0-based (posting 0 is the first posting).

### Common patterns

```bash
# All Electronics transactions in August 2025
queryledger -account Electronics -date 2025/08/01:2025/08/31 transactions.ledger

# Amazon charges excluding PC build parts
queryledger -account Other -payee Amazon -exclude-payee 'PC build' -date 2025/01/01:2025/12/31 transactions.ledger

# Get ref for a specific transaction
queryledger -csv ref -date 2025/09/19 -amount '-479.24' -file 2025.ledger transactions.ledger

# CSV with multi-posting detail
queryledger -csv date,description,account:0,amount:0,account:1,amount:1 -account Electronics transactions.ledger

# Cleared transactions only
queryledger -status clear -date 2025/01/01:2025/12/31 transactions.ledger

# Uncategorized charges still pending
queryledger -status pending -account Expenses:Uncategorized transactions.ledger
```

---

## Edit Tool (`editledger`)

Modify transactions by ref handle. Each edit creates a backup tarball before applying.

```bash
editledger -ref N:hash [-file file.ledger] [-set Field=Value ...] <ledger-file>
```

Output: the new ref for the edited transaction (print to stdout, capture for chaining).

### Editable fields

| Field | Syntax | Description |
|---|---|---|
| `description` | `-set 'description=New Payee'` | Transaction payee/description |
| `date` | `-set 'date=2025/08/20'` | Transaction date |
| `clear_date` | `-set 'clear_date=2025/08/22'` | Clearance date |
| `status` | `-set 'status=clear'` | `clear` or `*`, `pending` or `!`, `none` or empty |
| `code` | `-set 'code=TX123'` | Transaction code |
| `account` | `-set 'account=Expenses:Food'` | Posting 0 account (same as account:0) |
| `account:N` | `-set 'account:0=Expenses:Electronics'` | Specific posting account |
| `amount` | `-set 'amount=$20.00'` | Posting 0 amount (same as amount:0) |
| `amount:N` | `-set 'amount:1=$-20.00'` | Specific posting amount |
| `assert` | `-set 'assert=$-135.08'` | Balance assertion on posting 0 |
| `assert:N` | `-set 'assert:1='` | Clear assertion (empty value) |
| `note` | `-set 'note=Kitchen supplies'` | Note on posting 0 |
| `note:N` | `-set 'note:1=Credit card charge'` | Specific posting note |

### Adding and removing postings

```bash
# Insert a new posting at index 1 (shifts existing postings down)
editledger -ref N:hash -set 'posting:1=Expenses:Guns' -set 'amount:1=$10.00' ...

# Delete posting 1 (shifts remaining postings up)
editledger -ref N:hash -set 'posting:1=' ...

# Append a new posting at the end (N = current posting count)
editledger -ref N:hash -set 'posting:2=Expenses:Food' -set 'amount:2=$15.00' ...
```

### Common patterns

```bash
# Change account on posting 0
REF=$(queryledger -csv ref -date 2025/09/19 -amount '-479.24' -file 2025.ledger transactions.ledger)
editledger -ref "$REF" -set 'account:0=Expenses:Electronics' -file 2025.ledger transactions.ledger

# Multi-field edit in one call
editledger -ref "$REF" \
  -set 'description=Amazon Return (CPU)' \
  -set 'account:0=Expenses:Electronics' \
  -file 2025.ledger transactions.ledger

# Split a batched charge: insert postings, then remove original
REF=$(queryledger -csv ref -date 2025/08/20 -amount '1267.26' -file 2025.ledger transactions.ledger)
REF1=$(editledger -ref "$REF" -set 'posting:1=Expenses:Electronics' -set 'amount:1=$400.00' -file 2025.ledger transactions.ledger)
REF2=$(editledger -ref "$REF1" -set 'posting:2=Expenses:Food' -set 'amount:2=$867.26' -file 2025.ledger transactions.ledger)
# Then update posting 0 amount and remove original category if needed
```

### Ground rules

- Chain refs: capture stdout of each edit and pass to the next.
- Ref sequence numbers are stable when file position doesn't change; only the hash updates on content change.
- Backups are automatic — every edit creates a tarball.

---

## Pretty Printer (`fmtledger`)

Standardizes ledger file formatting.

```bash
fmtledger <ledger-file>
```

No flags. Formats all included ledger files in place, creating a timestamped `backup-*.tar.gz` when changes are made.
