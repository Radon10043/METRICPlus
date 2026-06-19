package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Radon10043/METRICPlus/src/helper"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/term"
)

var version = "dev"

const (
	metarelTable = "metarel"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[1;36m"
	colorYellow = "\033[33m"
	colorSelect = "\033[1;30;46m"
	colorDim    = "\033[2m"
)

type decision int

const (
	decisionNo decision = iota
	decisionYes
	decisionQuit
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "help":
			return runIdentify([]string{"-h"}, stdin, stdout, stderr)
		}
	}
	return runIdentify(args, stdin, stdout, stderr)
}

func runIdentify(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("METRICPlus", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printUsage(stdout)
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Options:")
		fs.SetOutput(stdout)
		fs.PrintDefaults()
		fs.SetOutput(stderr)
	}

	specFile := fs.String("spec", "", "YAML specification file (required)")
	dbFile := fs.String("out", "", "sqlite3 database file path (required)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n\n", fs.Arg(0))
		printUsage(stderr)
		return 2
	}
	if *specFile == "" || *dbFile == "" {
		fmt.Fprintln(stderr, "missing required option: -spec and -out must both be provided")
		printUsage(stderr)
		return 2
	}

	spec, err := loadYAMLSpecification(*specFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	store, err := openMRStore(*dbFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer store.Close()

	identifiedPairs, err := store.IdentifiedPairs()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	frames := spec.Frames
	pairs := helper.CandidatePairs(frames, "all", 0)
	if len(pairs) == 0 {
		fmt.Fprintln(stdout, "No candidate pairs were produced.")
		return 0
	}

	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	profile := spec.Profile
	completed := countKnownCandidatePairs(pairs, identifiedPairs)
	inspected := 0
	identifiedMRs := 0

	fmt.Fprintf(stdout, "%sMETRIC+ MR Identification%s\n", colorCyan, colorReset)
	fmt.Fprintf(stdout, "Specification: %s\n", spec.Source)
	fmt.Fprintf(stdout, "Database: %s\n", store.Path)
	fmt.Fprintf(stdout, "Frames: %d, candidate pairs: %d, already identified: %d\n", len(frames), len(pairs), completed)
	fmt.Fprintln(stdout, "For each pair, decide whether it is a metamorphic relation.")
	fmt.Fprintln(stdout, "Use Left/Right to select Y/N, Enter to confirm, Q or Ctrl-C to quit.")

	if completed == len(pairs) {
		fmt.Fprintln(stdout, "All candidate pairs have already been identified.")
		return 0
	}

	for _, pair := range pairs {
		sourceCTF := pair.SourceFrame.Choices.String()
		followCTF := pair.FollowUpFrame.Choices.String()
		pairKey := decisionKey(sourceCTF, followCTF)
		if identifiedPairs[pairKey] {
			continue
		}
		mrDecision, err := chooseDecision(scanner, stdin, stdout, profile, pair, completed+1, len(pairs), "Is it a metamorphic relation?")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if mrDecision == decisionQuit {
			return 0
		}
		res := 0
		if mrDecision == decisionYes {
			res = 1
			identifiedMRs++
		}

		if err := store.InsertDecision(sourceCTF, followCTF, res); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		identifiedPairs[pairKey] = true
		completed++
		inspected++
		if res == 1 {
			fmt.Fprintln(stdout, "Recorded MR.")
		}
	}

	fmt.Fprintf(stdout, "\nInspected pairs this run: %d\n", inspected)
	fmt.Fprintf(stdout, "Identified MRs this run: %d\n", identifiedMRs)
	fmt.Fprintf(stdout, "Stored decisions in %s, table %q.\n", store.Path, metarelTable)
	return 0
}

type loadedSpecification struct {
	Source  string
	Profile helper.Profile
	Frames  []helper.IOCTF
}

func loadYAMLSpecification(specFile string) (loadedSpecification, error) {
	ext := strings.ToLower(filepath.Ext(specFile))
	if ext != ".yaml" && ext != ".yml" {
		return loadedSpecification{}, fmt.Errorf("only YAML spec files are supported: %s", specFile)
	}
	spec, err := helper.LoadYAMLSpecification(specFile)
	if err != nil {
		return loadedSpecification{}, err
	}
	return loadedSpecification{Source: specFile, Profile: spec.Profile, Frames: spec.Frames}, nil
}

type mrStore struct {
	Path string
	db   *sql.DB
}

func openMRStore(dbFile string) (*mrStore, error) {
	dbPath, err := prepareDBPath(dbFile)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	store := &mrStore{Path: dbPath, db: db}
	if err := store.EnsureSchema(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func prepareDBPath(dbFile string) (string, error) {
	if dbFile == ":memory:" {
		return dbFile, nil
	}
	absDB, err := filepath.Abs(dbFile)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absDB), 0755); err != nil {
		return "", err
	}
	return absDB, nil
}

func (s *mrStore) EnsureSchema() error {
	_, err := s.db.Exec("CREATE TABLE IF NOT EXISTS " + metarelTable + " (id INTEGER PRIMARY KEY AUTOINCREMENT, ctf1 TEXT, ctf2 TEXT, res INTEGER)")
	return err
}

func (s *mrStore) IdentifiedPairs() (map[string]bool, error) {
	rows, err := s.db.Query("SELECT ctf1, ctf2 FROM " + metarelTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var ctf1, ctf2 string
		if err := rows.Scan(&ctf1, &ctf2); err != nil {
			return nil, err
		}
		out[decisionKey(ctf1, ctf2)] = true
	}
	return out, rows.Err()
}

func (s *mrStore) InsertDecision(sourceCTF, followCTF string, res int) error {
	_, err := s.db.Exec("INSERT INTO "+metarelTable+" (ctf1, ctf2, res) VALUES (?, ?, ?)", sourceCTF, followCTF, res)
	return err
}

func (s *mrStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func countKnownCandidatePairs(pairs []helper.CandidatePair, known map[string]bool) int {
	count := 0
	for _, pair := range pairs {
		if known[decisionKey(pair.SourceFrame.Choices.String(), pair.FollowUpFrame.Choices.String())] {
			count++
		}
	}
	return count
}

func decisionKey(sourceCTF, followCTF string) string {
	return sourceCTF + "\x00" + followCTF
}

func chooseDecision(scanner *bufio.Scanner, stdin io.Reader, stdout io.Writer, profile helper.Profile, pair helper.CandidatePair, current, total int, question string) (decision, error) {
	if !isInteractiveTerminal(stdin, stdout) {
		printPair(stdout, profile, pair, current, total, question, false)
		answer := ask(scanner, stdout, question+" [y/N/q]: ")
		switch answer {
		case "q":
			return decisionQuit, nil
		case "y", "yes":
			return decisionYes, nil
		default:
			return decisionNo, nil
		}
	}

	inFile := stdin.(*os.File)
	selectedYes := true
	for {
		clearScreen(stdout)
		printPair(stdout, profile, pair, current, total, question, selectedYes)
		key, err := readTerminalKey(inFile)
		if err != nil {
			return decisionQuit, err
		}
		switch key {
		case "left":
			selectedYes = true
		case "right":
			selectedYes = false
		case "enter":
			if selectedYes {
				return decisionYes, nil
			}
			return decisionNo, nil
		case "quit":
			return decisionQuit, nil
		}
	}
}

func readTerminalKey(file *os.File) (string, error) {
	oldState, err := term.MakeRaw(int(file.Fd()))
	if err != nil {
		return "", err
	}
	key, readErr := readKey(file)
	restoreErr := term.Restore(int(file.Fd()), oldState)
	if readErr != nil {
		return "", readErr
	}
	return key, restoreErr
}

func readKey(r io.Reader) (string, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return "", err
	}
	switch b[0] {
	case '\r', '\n':
		return "enter", nil
	case 'q', 'Q', 0x03:
		return "quit", nil
	case '\x1b':
		var seq [2]byte
		if _, err := io.ReadFull(r, seq[:]); err != nil {
			return "", err
		}
		if seq[0] == '[' {
			switch seq[1] {
			case 'D':
				return "left", nil
			case 'C':
				return "right", nil
			}
		}
	}
	return "", nil
}

func isInteractiveTerminal(stdin io.Reader, stdout io.Writer) bool {
	inFile, inOK := stdin.(*os.File)
	outFile, outOK := stdout.(*os.File)
	return inOK && outOK && isTerminal(inFile) && isTerminal(outFile)
}

func ask(scanner *bufio.Scanner, stdout io.Writer, prompt string) string {
	fmt.Fprint(stdout, prompt)
	if !scanner.Scan() {
		return "q"
	}
	return strings.ToLower(strings.TrimSpace(scanner.Text()))
}

func printPair(w io.Writer, profile helper.Profile, pair helper.CandidatePair, current, total int, question string, selectedYes bool) {
	fmt.Fprintf(w, "%sMETRIC+ MR Identification%s\n", colorCyan, colorReset)
	fmt.Fprintf(w, "%sCandidate%s %d / %d    %s -> %s    %s%s%s\n\n",
		colorDim, colorReset, current, total, pair.SourceFrame.Name(), pair.FollowUpFrame.Name(), colorDim, pair.RelationKind, colorReset)

	left := frameRows(profile, pair.SourceFrame)
	right := frameRows(profile, pair.FollowUpFrame)
	widths := tableWidths(left, right)
	sep := tableSeparator(widths)
	gap := "      "
	arrowGap := "  --> "

	fmt.Fprintln(w, sep+gap+sep)
	fmt.Fprintln(w, tableHeader(widths, "SOURCE")+arrowGap+tableHeader(widths, "FOLLOW-UP"))
	fmt.Fprintln(w, sep+gap+sep)

	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	prevIO := ""
	for i := 0; i < n; i++ {
		lrow := rowAt(left, i)
		rrow := rowAt(right, i)
		if i > 0 && lrow[0] != "" && lrow[0] != prevIO {
			fmt.Fprintln(w, sep+gap+sep)
		}
		prevIO = lrow[0]
		diff := !sameChoice(pair.SourceFrame.Choices, pair.FollowUpFrame.Choices, i)
		fmt.Fprintln(w, tableRow(widths, lrow, diff)+gap+tableRow(widths, rrow, diff))
	}
	fmt.Fprintln(w, sep+gap+sep)
	fmt.Fprintln(w)
	printDecisionBox(w, question, current, total, selectedYes)
}

func frameRows(profile helper.Profile, frame helper.IOCTF) [][3]string {
	rows := make([][3]string, 0, len(frame.Choices))
	for _, raw := range frame.Choices {
		rows = append(rows, profile.ChoiceColumns(raw))
	}
	return rows
}

func printDecisionBox(w io.Writer, question string, current, total int, selectedYes bool) {
	questionTitle := " " + question + " "
	choices, choicesWidth := decisionChoices(selectedYes)
	quitTitle := " [ Left/Right: select | Enter: confirm | Q/Ctrl-C: quit ] "
	progressTitle := " Progress "
	progress := fmt.Sprintf("%d/%d", current, total)

	leftWidth := maxInt(maxLen(questionTitle, quitTitle)+4, choicesWidth)
	progressWidth := maxLen(progressTitle, progress) + 2

	fmt.Fprintln(w, "+"+borderTitle(questionTitle, leftWidth)+"+"+borderTitle(progressTitle, progressWidth)+"+")
	fmt.Fprintln(w, "|"+choices+strings.Repeat(" ", leftWidth-choicesWidth)+"|"+cellText(progress, progressWidth, true)+"|")
	fmt.Fprintln(w, "+"+borderTitle(quitTitle, leftWidth)+"+"+strings.Repeat("-", progressWidth)+"+")
}

func decisionChoices(selectedYes bool) (string, int) {
	yes := "[Y] YES"
	no := "[N] NO"
	if selectedYes {
		yes = colorSelect + yes + colorReset
	} else {
		no = colorSelect + no + colorReset
	}
	return "        " + yes + "        " + no + "        ", len("        [Y] YES        [N] NO        ")
}

func borderTitle(title string, width int) string {
	if len(title) >= width {
		return title
	}
	left := (width - len(title)) / 2
	right := width - len(title) - left
	return strings.Repeat("-", left) + title + strings.Repeat("-", right)
}

func cellText(value string, width int, center bool) string {
	if len(value) > width {
		return truncate(value, width)
	}
	space := width - len(value)
	if center {
		left := space / 2
		right := space - left
		return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
	}
	return value + strings.Repeat(" ", space)
}

func maxLen(values ...string) int {
	max := 0
	for _, value := range values {
		if len(value) > max {
			max = len(value)
		}
	}
	return max
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tableWidths(left, right [][3]string) [3]int {
	widths := [3]int{5, len("Category  "), len("Choice  ")}
	for _, rows := range [][][3]string{left, right} {
		for _, row := range rows {
			for i := range widths {
				if l := len(row[i]) + 2; l > widths[i] {
					widths[i] = l
				}
			}
		}
	}
	if widths[1] > 14 {
		widths[1] = 14
	}
	if widths[2] > 44 {
		widths[2] = 44
	}
	return widths
}

func tableSeparator(widths [3]int) string {
	return "+" + strings.Repeat("-", widths[0]) + "+" + strings.Repeat("-", widths[1]) + "+" + strings.Repeat("-", widths[2]) + "+"
}

func tableHeader(widths [3]int, title string) string {
	return "|" + pad("I/O", widths[0], true) + "|" + pad("Category", widths[1], false) + "|" + pad(title+" Choice", widths[2], false) + "|"
}

func tableRow(widths [3]int, row [3]string, highlight bool) string {
	choice := pad(row[2], widths[2], false)
	if highlight {
		choice = colorYellow + choice + colorReset
	}
	return "|" + pad(row[0], widths[0], true) + "|" + pad(row[1], widths[1], false) + "|" + choice + "|"
}

func rowAt(rows [][3]string, idx int) [3]string {
	if idx < 0 || idx >= len(rows) {
		return [3]string{"", "", ""}
	}
	return rows[idx]
}

func sameChoice(left, right helper.ChoiceList, idx int) bool {
	if idx < 0 || idx >= len(left) || idx >= len(right) {
		return false
	}
	return left[idx] == right[idx]
}

func pad(value string, width int, center bool) string {
	value = truncate(value, width-1)
	if len(value) >= width {
		return value
	}
	space := width - len(value)
	if center {
		left := space / 2
		right := space - left
		return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
	}
	return value + strings.Repeat(" ", space)
}

func truncate(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	return value[:width-1] + "~"
}

func clearScreen(w io.Writer) {
	fmt.Fprint(w, "\033[2J\033[H")
}

func shouldClear(stdin io.Reader, stdout io.Writer) bool {
	inFile, inOK := stdin.(*os.File)
	outFile, outOK := stdout.(*os.File)
	if !inOK || !outOK {
		return false
	}
	return isTerminal(inFile) && isTerminal(outFile)
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `METRIC+ in Go

Usage:
  METRICPlus -spec <spec.yaml> -out <result.db>

Examples:
  METRICPlus -spec data/fastjson.yaml -out data/fastjson.db

SQLite storage:
  table metarel(id INTEGER PRIMARY KEY AUTOINCREMENT, ctf1 TEXT, ctf2 TEXT, res INTEGER)
  res = 1 means the pair is identified as an MR, res = 0 means it is not.`)
}
