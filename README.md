# Markdown Front Matter Validator (mdfmv)

A tiny CLI to scan Markdown files and validate their front matter against a JSON Schema.

## Motivation

Existing front-matter validation tools require runtimes or external dependencies, which makes them awkward for simple, local checks or lightweight CI jobs. `mdfmv` is intentionally tiny and self-contained - no runtime, zero dependencies, no external services - making it fast, portable, and suitable for offline use and automated pipelines.

## Install

### Manual

- Download from [releases](https://github.com/dariuszporowski/mdfmv/releases).
- Add executable somewhere in your path depending on your platform.

### Source

```shell
go install github.com/dariuszporowski/mdfmv@latest
```

## Usage

```text
mdfmv [path] [flags]
```

Flags (global):

- `-s, --schema` Path or URL to a JSON Schema for all files
- `-r, --recursive` Scan recursively (default true)
- `-e, --ext` File extensions to include (default .md, .markdown, .mdown, .mdx)
- `-q, --quiet` Quiet mode; only print errors
- `-o, --output` Output format: json, tsv, yaml (default tsv)
- `--force` Fail files that do not contain front matter
- `--base-dir` Base directory to resolve relative $schema when reading from stdin

## Schema sources

mdfmv can get a schema in three ways (precedence listed):

1. `--schema` flag: a file path or http(s) URL
1. Front matter key (`$schema`) with:
   - a string value pointing to a file path or URL
   - an inline JSON/YAML object containing the schema
1. If none found, file is treated as OK with a `"no schema provided"` warning

## Examples

### Global schema via flag

```shell
mdfmv . --schema ./.examples/schema.json
```

### Per-file schema reference in front matter

```markdown
---
$schema: ../schemas/post.schema.json
slug: hello-world
title: Hello World
date: 2025-08-14
---

Content...
```

### Inline schema in front matter

```markdown
---
$schema:
  $schema: "https://json-schema.org/draft/2020-12/schema"
  type: object
  required: [title]
  properties:
    title: { type: string }
---

title: Hello
```

## Exit codes

- 0: All files valid
- 1: One or more files failed schema validation
- 2: Files has only warnings

## Notes

- Current front matter parser supports YAML delimited by `---`.
- JSON front matter fences can be added later.
- Ignores respect `.mdfmvignore` (gitignore syntax) and `.gitignore` at the scan root.

### Using stdin (no path argument)

You can pipe a single Markdown document to `mdfmv`. When piping, omit the path argument. If the front matter references a relative `$schema`, use `--base-dir` to set the directory for resolving it.

```powershell
Get-Content .\.examples\ok.md | mdfmv --output json --base-dir .\.examples
```

```bash
cat ./examples/ok.md | mdfmv --output yaml --base-dir ./.examples
```

## Build

- Requires Go 1.25+
- From source in this repo:

```shell
# Build
go build .
# Or run without building
go run . --help
```
