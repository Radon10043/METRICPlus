# METRIC+

METRIC+ is a Go CLI that reproduces the metamorphic relation identification workflow described in the METRIC+ paper.

**NOTE: This is unofficial implementation of METRIC+**

## Project Status

This repository is a reproduction of the paper *METRIC+: A Metamorphic Relation Identification Technique Based on Input Plus Output Domains*. It is not an official implementation from the paper authors, and it should be treated as a practical reimplementation for study, inspection, and experimentation.

The goal is to make the core MR identification workflow easy to build, run, and inspect from a terminal. The implementation uses a simplified YAML specification format and stores identification decisions in sqlite3.

The implemented scope is intentionally narrow:

- model IO-CTFs with input choices and output choices
- group IO-CTFs by identical output test frames
- enumerate output-guided candidate pairs within and across output groups
- walk the user through MR decisions in the terminal
- record MR identification decisions in sqlite3

This project does not reproduce the full experimental infrastructure around METRIC+. In particular, it does not implement test generation, SUT execution, coverage collection, or output-oracle validation.

## Build

```bash
make
```

The binary is written to `bin/METRICPlus`:

```bash
bin/METRICPlus
```

The Makefile uses `go` from `PATH`, or `/usr/local/go/bin/go` when `go` is not on `PATH`.

All Go source code is under `src/`; `go.mod` stays at the project root.

## Use

Run the identification workflow with a YAML specification and an explicit sqlite3 output path:

```bash
./bin/METRICPlus -spec data/fastjson.yaml -out data/fastjson.db
```

The only supported options are `-spec` and `-out`. Both are required and have no default value. `METRICPlus` reads YAML specs only.

The sqlite3 database specified by `-out` stores identification decisions in the following table:

```sql
CREATE TABLE IF NOT EXISTS metarel (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ctf1 TEXT,
  ctf2 TEXT,
  res INTEGER
);
```

`ctf1` and `ctf2` are comma-separated complete test frames. `res = 1` means the pair is identified as an MR; `res = 0` means it is not. Existing rows are treated as completed decisions and skipped on the next run.

## Specification Format

The spec is a YAML file containing the category-choice profile and complete test frames needed by the terminal UI:

```yaml
profile:
  categories:
    I:
      "1": JsonFormat
    O:
      "1": Exception
  choices:
    I:
      "1":
        "1": Object
    O:
      "1":
        "1": No exception raised

frames:
  - I-1-1,O-1-1
```

`profile` controls the category and choice text printed in the terminal table. Complete profile names make the MR identification view readable in the terminal.

`choices` can also be written as a YAML list when that is clearer:

```yaml
frames:
  - - I-1-1
    - O-1-1
```

Frame entries may also be written as `- choices: ...` when compatibility with that YAML shape is needed.

Each choice uses the `I-category-choice` or `O-category-choice` form. `METRICPlus` treats each complete test frame as an IO-CTF and uses the O-choices to reduce and order the MR identification search space.

## Acknowledgements

Thanks to Chang-ai Sun, An Fu, and collaborators for the METRIC+ research.

```bibtex
@ARTICLE{8807231,
  author    = {Sun, Chang-Ai and Fu, An and Poon, Pak-Lok and Xie, Xiaoyuan and Liu, Huai and Chen, Tsong Yueh},
  journal   = {IEEE Transactions on Software Engineering}, 
  title     = {METRIC$^{+}$+: A Metamorphic Relation Identification Technique Based on Input Plus Output Domains}, 
  year      = {2021},
  volume    = {47},
  number    = {9},
  pages     = {1764-1785},
  keywords  = {Measurement;Testing;Software systems;Fault detection;Task analysis;Tools;Metamorphic testing;metamorphic relation;category-choice framework;fault detection effectiveness},
  doi       = {10.1109/TSE.2019.2934848}
}
```
