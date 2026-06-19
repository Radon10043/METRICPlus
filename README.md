# METRIC+

METRIC+ is a Go CLI for metamorphic relation identification.

This project focuses only on MR identification:

- model IO-CTFs with input choices and output choices
- group IO-CTFs by identical output test frames
- enumerate output-guided candidate pairs within and across output groups
- walk the user through MR decisions in the terminal
- record MR identification decisions in sqlite3

## Build

```bash
make
```

The binary is written to:

```bash
bin/METRICPlus
```

The Makefile uses `go` from `PATH`, or `/usr/local/go/bin/go` when `go` is not on `PATH`.

All Go source code is under `src/`; `go.mod` stays at the project root.

## Use

Start the terminal identification workflow with a YAML specification and an explicit sqlite3 output path:

```bash
./bin/METRICPlus -spec data/fastjson.yaml -out data/fastjson.db
```

The only supported options are `-spec` and `-out`. Both are required and have no default value. METRICPlus reads YAML specs only.

Identified decisions are stored in the sqlite3 database specified by `-out`.

METRIC+ stores decisions in the following sqlite3 table:

```sql
CREATE TABLE IF NOT EXISTS metarel (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ctf1 TEXT,
  ctf2 TEXT,
  res INTEGER
);
```

`ctf1` and `ctf2` are comma-separated complete test frames. `res = 1` means the pair is identified as an MR; `res = 0` means it is not. Existing rows are treated as already identified and skipped on the next run.

## Specification Format

The spec is a YAML file with the information METRICPlus needs for terminal rendering:

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

Each choice uses the `I-category-choice` or `O-category-choice` form. METRIC+ treats each complete test frame as an IO-CTF and uses the O-choices to reduce and order the MR identification search space.

## Acknowledgements

Thanks to Chang-ai Sun, An Fu, and collaborators for their work on METRIC+.
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
