package main

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Radon10043/METRICPlus/src/helper"
	_ "github.com/mattn/go-sqlite3"
)

const metarelSchema = "CREATE TABLE IF NOT EXISTS metarel (id INTEGER PRIMARY KEY AUTOINCREMENT, ctf1 TEXT, ctf2 TEXT, res INTEGER)"

type legacyDecision struct {
	PairName string
	Group1   string
	Group2   string
	HasMR    string
	InputRel string
}

type importStats struct {
	InputPairs         int
	RowsInserted       int
	MRs                int
	NonMRs             int
	DuplicatesSkipped  int
	RelationMismatches int
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("legacydb", flag.ContinueOnError)
	dataDir := fs.String("data", "data", "directory containing legacy MR files and YAML specs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	files, err := legacyFiles(*dataDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no legacy .xml or .json files with matching .yaml specs found in %s", *dataDir)
	}

	for _, file := range files {
		base := strings.TrimSuffix(file, filepath.Ext(file))
		specFile := base + ".yaml"
		dbFile := base + ".db"
		stats, err := convertOne(specFile, file, dbFile)
		if err != nil {
			return err
		}
		fmt.Printf("%s -> %s: %d rows (%d MR, %d non-MR), %d input pairs",
			file, dbFile, stats.RowsInserted, stats.MRs, stats.NonMRs, stats.InputPairs)
		if stats.DuplicatesSkipped > 0 {
			fmt.Printf(", %d duplicate pairs skipped", stats.DuplicatesSkipped)
		}
		if stats.RelationMismatches > 0 {
			fmt.Printf(", %d input relation mismatches", stats.RelationMismatches)
		}
		fmt.Println()
	}
	return nil
}

func legacyFiles(dataDir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dataDir, "*.*"))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(matches))
	for _, file := range matches {
		ext := strings.ToLower(filepath.Ext(file))
		if ext != ".xml" && ext != ".json" {
			continue
		}
		base := strings.TrimSuffix(file, filepath.Ext(file))
		if _, err := os.Stat(base + ".yaml"); err == nil {
			out = append(out, file)
		}
	}
	sort.Strings(out)
	return out, nil
}

func convertOne(specFile, legacyFile, dbFile string) (importStats, error) {
	spec, err := helper.LoadYAMLSpecification(specFile)
	if err != nil {
		return importStats{}, err
	}
	decisions, err := loadLegacyDecisions(legacyFile)
	if err != nil {
		return importStats{}, err
	}
	groupIndex := outputGroupIndex(spec.Frames)
	candidates := candidateKeySet(spec.Frames)

	if err := os.Remove(dbFile); err != nil && !os.IsNotExist(err) {
		return importStats{}, err
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return importStats{}, err
	}
	defer db.Close()

	if _, err := db.Exec(metarelSchema); err != nil {
		return importStats{}, err
	}
	tx, err := db.Begin()
	if err != nil {
		return importStats{}, err
	}
	stmt, err := tx.Prepare("INSERT INTO metarel (ctf1, ctf2, res) VALUES (?, ?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return importStats{}, err
	}
	defer stmt.Close()

	stats := importStats{InputPairs: len(decisions)}
	seen := map[string]int{}
	for _, decision := range decisions {
		source, follow, err := resolveFrames(decision, groupIndex)
		if err != nil {
			_ = tx.Rollback()
			return importStats{}, fmt.Errorf("%s: %w", legacyFile, err)
		}
		ctf1 := source.Choices.String()
		ctf2 := follow.Choices.String()
		key := ctf1 + "\x00" + ctf2
		res := 0
		if strings.EqualFold(strings.TrimSpace(decision.HasMR), "Yes") {
			res = 1
		}
		if previous, ok := seen[key]; ok {
			if previous != res {
				_ = tx.Rollback()
				return importStats{}, fmt.Errorf("%s: conflicting duplicate decision for %q -> %q", legacyFile, ctf1, ctf2)
			}
			stats.DuplicatesSkipped++
			continue
		}
		if !candidates[key] {
			_ = tx.Rollback()
			return importStats{}, fmt.Errorf("%s: legacy pair is not a current METRIC+ candidate: %q -> %q", legacyFile, ctf1, ctf2)
		}
		if res == 1 && !inputRelationMatches(decision.InputRel, source, follow) {
			stats.RelationMismatches++
		}
		if _, err := stmt.Exec(ctf1, ctf2, res); err != nil {
			_ = tx.Rollback()
			return importStats{}, err
		}
		seen[key] = res
		stats.RowsInserted++
		if res == 1 {
			stats.MRs++
		} else {
			stats.NonMRs++
		}
	}
	if err := tx.Commit(); err != nil {
		return importStats{}, err
	}
	if stats.RelationMismatches > 0 {
		return stats, fmt.Errorf("%s: %d mapped MR pairs do not match inputrelationdefinition", legacyFile, stats.RelationMismatches)
	}
	return stats, nil
}

func outputGroupIndex(frames []helper.IOCTF) map[string][]helper.IOCTF {
	out := map[string][]helper.IOCTF{}
	for _, frame := range frames {
		out[frame.OutputKey()] = append(out[frame.OutputKey()], frame)
	}
	return out
}

func candidateKeySet(frames []helper.IOCTF) map[string]bool {
	out := map[string]bool{}
	for _, pair := range helper.CandidatePairs(frames, "all", 0) {
		out[pair.SourceFrame.Choices.String()+"\x00"+pair.FollowUpFrame.Choices.String()] = true
	}
	return out
}

func resolveFrames(decision legacyDecision, groups map[string][]helper.IOCTF) (helper.IOCTF, helper.IOCTF, error) {
	leftIndex, rightIndex, err := pairIndexes(decision.PairName)
	if err != nil {
		return helper.IOCTF{}, helper.IOCTF{}, err
	}
	group1Key, err := legacyOutputKey(decision.Group1)
	if err != nil {
		return helper.IOCTF{}, helper.IOCTF{}, err
	}
	group2Key, err := legacyOutputKey(decision.Group2)
	if err != nil {
		return helper.IOCTF{}, helper.IOCTF{}, err
	}
	leftFrames, ok := groups[group1Key]
	if !ok {
		return helper.IOCTF{}, helper.IOCTF{}, fmt.Errorf("output group %q not found in YAML spec", decision.Group1)
	}
	rightFrames, ok := groups[group2Key]
	if !ok {
		return helper.IOCTF{}, helper.IOCTF{}, fmt.Errorf("output group %q not found in YAML spec", decision.Group2)
	}
	if leftIndex < 0 || leftIndex >= len(leftFrames) {
		return helper.IOCTF{}, helper.IOCTF{}, fmt.Errorf("left pair index %d out of range for %q", leftIndex, decision.Group1)
	}
	if rightIndex < 0 || rightIndex >= len(rightFrames) {
		return helper.IOCTF{}, helper.IOCTF{}, fmt.Errorf("right pair index %d out of range for %q", rightIndex, decision.Group2)
	}
	return leftFrames[leftIndex], rightFrames[rightIndex], nil
}

func pairIndexes(pairName string) (int, int, error) {
	parts := strings.Split(pairName, "<->")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid pairname %q", pairName)
	}
	left, err := sideIndex(parts[0])
	if err != nil {
		return 0, 0, err
	}
	right, err := sideIndex(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return left, right, nil
}

func sideIndex(side string) (int, error) {
	side = strings.TrimSpace(side)
	pos := strings.LastIndex(side, "}:")
	if pos < 0 {
		return 0, fmt.Errorf("invalid pair side %q", side)
	}
	raw := strings.TrimSpace(side[pos+2:])
	idx, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid pair index %q: %w", raw, err)
	}
	return idx, nil
}

func legacyOutputKey(group string) (string, error) {
	choices, err := legacyChoiceList(group, "O")
	if err != nil {
		return "", err
	}
	sort.Strings(choices)
	return strings.Join(choices, ","), nil
}

func legacyInputKey(side string) (string, error) {
	choices, err := legacyChoiceList(side, "I")
	if err != nil {
		return "", err
	}
	return strings.Join(choices, ","), nil
}

func legacyChoiceList(raw, ioPrefix string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "{")
	raw = strings.TrimSuffix(raw, "}")
	if raw == "" {
		return nil, fmt.Errorf("empty legacy choice list")
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for idx, part := range parts {
		choice, err := legacyChoice(strings.TrimSpace(part), ioPrefix, idx+1)
		if err != nil {
			return nil, err
		}
		out = append(out, choice)
	}
	return out, nil
}

func legacyChoice(raw, defaultIO string, position int) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty legacy choice")
	}
	if raw == "*" {
		return fmt.Sprintf("%s-%d-*", defaultIO, position), nil
	}
	if ref, err := helper.ParseChoiceRef(raw); err == nil {
		return ref.String(), nil
	}

	ioPrefix := defaultIO
	rest := raw
	if len(raw) > 2 && raw[1] == '-' {
		ioPrefix = strings.ToUpper(raw[:1])
		rest = raw[2:]
	}
	pos := 0
	for pos < len(rest) && rest[pos] >= '0' && rest[pos] <= '9' {
		pos++
	}
	if pos == 0 || pos == len(rest) {
		return "", fmt.Errorf("invalid legacy choice %q", raw)
	}
	return fmt.Sprintf("%s-%s-%s", ioPrefix, rest[:pos], rest[pos:]), nil
}

func inputRelationMatches(raw string, source, follow helper.IOCTF) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" {
		return true
	}
	parts := strings.Split(raw, "->")
	if len(parts) != 2 {
		return false
	}
	left, err := legacyInputKey(parts[0])
	if err != nil {
		return false
	}
	right, err := legacyInputKey(parts[1])
	if err != nil {
		return false
	}
	return relationSideMatches(left, source.Choices.Input()) && relationSideMatches(right, follow.Choices.Input())
}

func relationSideMatches(relation string, frame helper.ChoiceList) bool {
	relationRefs, err := helper.ChoiceList(strings.Split(relation, ",")).Refs()
	if err != nil {
		return false
	}
	frameRefs, err := frame.Refs()
	if err != nil {
		return false
	}

	relationByCategory := map[string]string{}
	for _, ref := range relationRefs {
		relationByCategory[ref.IO+"-"+ref.Category] = ref.Choice
	}
	frameByCategory := map[string]string{}
	for _, ref := range frameRefs {
		frameByCategory[ref.IO+"-"+ref.Category] = ref.Choice
	}
	for key, choice := range relationByCategory {
		if choice == "*" {
			continue
		}
		if frameByCategory[key] != choice {
			return false
		}
	}
	for _, ref := range frameRefs {
		choice, ok := relationByCategory[ref.IO+"-"+ref.Category]
		if !ok {
			return false
		}
		if choice != "*" && choice != ref.Choice {
			return false
		}
	}
	return true
}

func loadLegacyDecisions(file string) ([]legacyDecision, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(file)) {
	case ".xml":
		return loadXMLDecisions(data)
	case ".json":
		return loadJSONDecisions(data)
	default:
		return nil, fmt.Errorf("unsupported legacy file: %s", file)
	}
}

type xmlRoot struct {
	Diff xmlSection `xml:"pairsfromdiffgroups"`
	Same xmlSection `xml:"pairsfromsamegroups"`
}

type xmlSection struct {
	Groups []xmlGroup `xml:"group"`
}

type xmlGroup struct {
	Name  string    `xml:"name,attr"`
	Pairs []xmlPair `xml:"pair"`
}

type xmlPair struct {
	PairNameAttr string `xml:"pairname,attr"`
	PairNameElem string `xml:"pairname"`
	Group1       string `xml:"groupnameoftestframe1"`
	Group2       string `xml:"groupnameoftestframe2"`
	HasMR        string `xml:"hasmr"`
	InputRel     string `xml:"inputrelationdefinition"`
}

func loadXMLDecisions(data []byte) ([]legacyDecision, error) {
	var root xmlRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	out := []legacyDecision{}
	add := func(groups []xmlGroup) {
		for _, group := range groups {
			for _, pair := range group.Pairs {
				name := strings.TrimSpace(pair.PairNameElem)
				if name == "" {
					name = strings.TrimSpace(pair.PairNameAttr)
				}
				group1 := strings.TrimSpace(pair.Group1)
				group2 := strings.TrimSpace(pair.Group2)
				if group1 == "" || group2 == "" {
					group1, group2 = groupNamesFromPair(name)
				}
				out = append(out, legacyDecision{
					PairName: name,
					Group1:   group1,
					Group2:   group2,
					HasMR:    strings.TrimSpace(pair.HasMR),
					InputRel: strings.TrimSpace(pair.InputRel),
				})
			}
		}
	}
	add(root.Diff.Groups)
	add(root.Same.Groups)
	return out, nil
}

type jsonRoot struct {
	Diff []jsonGroup `json:"pairsfromdiffgroups"`
	Same []jsonGroup `json:"pairsfromsamegroups"`
}

type jsonGroup struct {
	Name  string     `json:"group name"`
	Pairs []jsonPair `json:"pairs"`
}

type jsonPair struct {
	PairName string `json:"pairname"`
	Group1   string `json:"groupnameoftestframe1"`
	Group2   string `json:"groupnameoftestframe2"`
	HasMR    string `json:"hasmr"`
	InputRel string `json:"inputrelationdefinition"`
}

func loadJSONDecisions(data []byte) ([]legacyDecision, error) {
	var root jsonRoot
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	out := []legacyDecision{}
	add := func(groups []jsonGroup) {
		for _, group := range groups {
			for _, pair := range group.Pairs {
				group1 := strings.TrimSpace(pair.Group1)
				group2 := strings.TrimSpace(pair.Group2)
				if group1 == "" || group2 == "" {
					group1, group2 = groupNamesFromPair(pair.PairName)
				}
				out = append(out, legacyDecision{
					PairName: strings.TrimSpace(pair.PairName),
					Group1:   group1,
					Group2:   group2,
					HasMR:    strings.TrimSpace(pair.HasMR),
					InputRel: strings.TrimSpace(pair.InputRel),
				})
			}
		}
	}
	add(root.Diff)
	add(root.Same)
	return out, nil
}

func groupNamesFromPair(pairName string) (string, string) {
	parts := strings.Split(pairName, "<->")
	if len(parts) != 2 {
		return "", ""
	}
	return groupNameFromSide(parts[0]), groupNameFromSide(parts[1])
}

func groupNameFromSide(side string) string {
	pos := strings.LastIndex(side, "}:")
	if pos < 0 {
		return ""
	}
	return strings.TrimSpace(side[:pos+1])
}
