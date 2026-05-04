# Dabkrs Examples

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![SQLite](https://img.shields.io/badge/SQLite-3-blue?style=flat&logo=sqlite)](https://sqlite.org)

Chinese dictionary enrichment tool using LLM to generate example sentences.

## Overview

Enriches Chinese dictionary entries with example sentences using a local LLM (Gemma 4).

## Usage

```bash
# Build
go build ./cmd/enrich

# Run (process dictionary)
go run ./cmd/enrich -example-batch 200 -batch 200 -limit 100

# Flags
  -db string        Source DB path (default: "dictionary.db")
  -example-batch int  Words per LLM request (default: 3)
  -batch int        DB write batch size (default: 20)
  -limit int        Max entries to process (0=all)
  -max-chars int    Skip entries longer than N chars (default: 2)
  -llm string       LLM API URL override
  -model string     LLM model override
```

## CLI Test Tool

```bash
# First 10 entries
go run ./cmd/test_db

# By headword
go run ./cmd/test_db -headword "你好"
```

## Tech Stack

- Go 1.25
- SQLite3
- Local LLM (Gemma 4 via LM Studio)