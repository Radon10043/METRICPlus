package helper

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultSpecPath    = "data/fastjson.spec.yaml"
	DefaultXLSXPath    = "data/fastjson.xlsx"
	DefaultChoiceSheet = "choice-detail"
	DefaultFrameSheet  = "complete-test-frames"
)

type Specification struct {
	Profile Profile `json:"profile" yaml:"profile"`
	Frames  []IOCTF `json:"frames" yaml:"frames"`
}

type yamlSpecification struct {
	Profile Profile     `yaml:"profile"`
	Frames  []yamlFrame `yaml:"frames"`
}

type yamlFrame struct {
	ID      string
	Choices ChoiceList
}

func (f *yamlFrame) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode, yaml.SequenceNode:
		if err := f.Choices.UnmarshalYAML(value); err != nil {
			return err
		}
		f.ID = ""
		return nil
	case yaml.MappingNode:
		var legacy IOCTF
		if err := value.Decode(&legacy); err != nil {
			return err
		}
		f.ID = legacy.ID
		f.Choices = legacy.Choices
		return nil
	default:
		return fmt.Errorf("frame must be a choice string, choice list, or mapping with choices")
	}
}

func (f yamlFrame) MarshalYAML() (any, error) {
	return f.Choices.MarshalYAML()
}

func LoadSpecification(file, choiceSheet, frameSheet string) (Specification, error) {
	book, err := openXLSX(file)
	if err != nil {
		return Specification{}, err
	}

	choiceRows, err := book.sheetRows(choiceSheet)
	if err != nil {
		return Specification{}, err
	}
	frameRows, err := book.sheetRows(frameSheet)
	if err != nil {
		return Specification{}, err
	}

	profile, err := profileFromRows(choiceRows)
	if err != nil {
		return Specification{}, err
	}
	frames, err := framesFromRows(frameRows)
	if err != nil {
		return Specification{}, err
	}
	return Specification{Profile: profile, Frames: frames}, nil
}

func LoadSpecificationFile(file, choiceSheet, frameSheet string) (Specification, error) {
	switch strings.ToLower(path.Ext(file)) {
	case ".json":
		return LoadJSONSpecification(file)
	case ".yaml", ".yml":
		return LoadYAMLSpecification(file)
	default:
		return LoadSpecification(file, choiceSheet, frameSheet)
	}
}

func LoadJSONSpecification(file string) (Specification, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Specification{}, err
	}
	var spec Specification
	if err := json.Unmarshal(data, &spec); err != nil {
		return Specification{}, err
	}
	if err := spec.Validate(); err != nil {
		return Specification{}, err
	}
	fillMissingFrameIDs(spec.Frames)
	return spec, nil
}

func LoadYAMLSpecification(file string) (Specification, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Specification{}, err
	}
	var yamlSpec yamlSpecification
	if err := yaml.Unmarshal(data, &yamlSpec); err != nil {
		return Specification{}, err
	}
	spec := fromYAMLSpecification(yamlSpec)
	if err := spec.Validate(); err != nil {
		return Specification{}, err
	}
	fillMissingFrameIDs(spec.Frames)
	return spec, nil
}

func MarshalSpecification(spec Specification, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(spec, "", "  ")
	case "yaml", "yml":
		return yaml.Marshal(toYAMLSpecification(spec))
	default:
		return nil, fmt.Errorf("unsupported spec format %q", format)
	}
}

func fromYAMLSpecification(in yamlSpecification) Specification {
	frames := make([]IOCTF, 0, len(in.Frames))
	for _, frame := range in.Frames {
		frames = append(frames, IOCTF{ID: frame.ID, Choices: frame.Choices})
	}
	return Specification{Profile: in.Profile, Frames: frames}
}

func toYAMLSpecification(spec Specification) yamlSpecification {
	frames := make([]yamlFrame, 0, len(spec.Frames))
	for _, frame := range spec.Frames {
		frames = append(frames, yamlFrame{ID: frame.ID, Choices: frame.Choices})
	}
	return yamlSpecification{Profile: spec.Profile, Frames: frames}
}

func fillMissingFrameIDs(frames []IOCTF) {
	for i := range frames {
		if frames[i].ID == "" {
			frames[i].ID = strconv.Itoa(i + 1)
		}
	}
}

func (s Specification) Validate() error {
	if len(s.Profile.Categories) == 0 {
		return fmt.Errorf("spec profile.categories is empty")
	}
	if len(s.Profile.Choices) == 0 {
		return fmt.Errorf("spec profile.choices is empty")
	}
	if len(s.Frames) == 0 {
		return fmt.Errorf("spec frames is empty")
	}
	return validateFrames(s.Frames)
}

type xlsxBook struct {
	files        map[string]*zip.File
	sharedString []string
	sheets       map[string]string
}

func openXLSX(file string) (*xlsxBook, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	book := &xlsxBook{
		files:  map[string]*zip.File{},
		sheets: map[string]string{},
	}
	for _, f := range reader.File {
		book.files[f.Name] = f
	}

	shared, err := book.readSharedStrings()
	if err != nil {
		return nil, err
	}
	book.sharedString = shared

	sheets, err := book.readWorkbookSheets()
	if err != nil {
		return nil, err
	}
	book.sheets = sheets
	return book, nil
}

func (b *xlsxBook) read(name string) ([]byte, error) {
	f, ok := b.files[name]
	if !ok {
		return nil, fmt.Errorf("xlsx entry %q not found", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (b *xlsxBook) readSharedStrings() ([]string, error) {
	if _, ok := b.files["xl/sharedStrings.xml"]; !ok {
		return nil, nil
	}
	data, err := b.read("xl/sharedStrings.xml")
	if err != nil {
		return nil, err
	}

	type textNode struct {
		Text string `xml:",chardata"`
	}
	type richRun struct {
		Text textNode `xml:"t"`
	}
	type stringItem struct {
		Text textNode  `xml:"t"`
		Runs []richRun `xml:"r"`
	}
	type sharedStrings struct {
		Items []stringItem `xml:"si"`
	}

	var ss sharedStrings
	if err := xml.Unmarshal(data, &ss); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ss.Items))
	for _, item := range ss.Items {
		if item.Text.Text != "" || len(item.Runs) == 0 {
			out = append(out, item.Text.Text)
			continue
		}
		var b strings.Builder
		for _, run := range item.Runs {
			b.WriteString(run.Text.Text)
		}
		out = append(out, b.String())
	}
	return out, nil
}

func (b *xlsxBook) readWorkbookSheets() (map[string]string, error) {
	type rel struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	}
	type relationships struct {
		Relations []rel `xml:"Relationship"`
	}

	relsData, err := b.read("xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}
	var rels relationships
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		return nil, err
	}
	targets := map[string]string{}
	for _, rel := range rels.Relations {
		target := rel.Target
		if !strings.HasPrefix(target, "xl/") {
			target = path.Clean("xl/" + target)
		}
		targets[rel.ID] = target
	}

	type sheet struct {
		Name string `xml:"name,attr"`
		RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	}
	type sheetsXML struct {
		Sheets []sheet `xml:"sheets>sheet"`
	}

	workbookData, err := b.read("xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	var workbook sheetsXML
	if err := xml.Unmarshal(workbookData, &workbook); err != nil {
		return nil, err
	}

	sheets := map[string]string{}
	for _, sheet := range workbook.Sheets {
		target, ok := targets[sheet.RID]
		if !ok {
			return nil, fmt.Errorf("worksheet relation %q for sheet %q not found", sheet.RID, sheet.Name)
		}
		sheets[sheet.Name] = target
	}
	return sheets, nil
}

func (b *xlsxBook) sheetRows(sheetName string) ([][]string, error) {
	entry, ok := b.sheets[sheetName]
	if !ok {
		names := make([]string, 0, len(b.sheets))
		for name := range b.sheets {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("sheet %q not found; available sheets: %s", sheetName, strings.Join(names, ", "))
	}
	data, err := b.read(entry)
	if err != nil {
		return nil, err
	}
	return parseWorksheet(data, b.sharedString)
}

func parseWorksheet(data []byte, shared []string) ([][]string, error) {
	type cell struct {
		Ref       string `xml:"r,attr"`
		Type      string `xml:"t,attr"`
		Value     string `xml:"v"`
		InlineStr struct {
			Text string `xml:"t"`
		} `xml:"is"`
	}
	type row struct {
		Cells []cell `xml:"c"`
	}
	type worksheet struct {
		Rows []row `xml:"sheetData>row"`
	}

	var ws worksheet
	if err := xml.Unmarshal(data, &ws); err != nil {
		return nil, err
	}

	rows := make([][]string, 0, len(ws.Rows))
	for _, row := range ws.Rows {
		vals := []string{}
		nextCol := 0
		for _, cell := range row.Cells {
			col := nextCol
			if cell.Ref != "" {
				col = columnIndex(cell.Ref)
			}
			for len(vals) <= col {
				vals = append(vals, "")
			}
			vals[col] = cellValue(cell.Type, cell.Value, cell.InlineStr.Text, shared)
			nextCol = col + 1
		}
		rows = append(rows, trimRight(vals))
	}
	return rows, nil
}

func cellValue(cellType, value, inline string, shared []string) string {
	switch cellType {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil && idx >= 0 && idx < len(shared) {
			return strings.TrimSpace(shared[idx])
		}
	case "inlineStr":
		return strings.TrimSpace(inline)
	}
	return strings.TrimSpace(value)
}

var colRefPattern = regexp.MustCompile(`^[A-Za-z]+`)

func columnIndex(ref string) int {
	letters := colRefPattern.FindString(ref)
	idx := 0
	for _, r := range strings.ToUpper(letters) {
		idx = idx*26 + int(r-'A'+1)
	}
	if idx == 0 {
		return 0
	}
	return idx - 1
}

func trimRight(vals []string) []string {
	end := len(vals)
	for end > 0 && strings.TrimSpace(vals[end-1]) == "" {
		end--
	}
	return vals[:end]
}

func profileFromRows(rows [][]string) (Profile, error) {
	profile := Profile{
		Categories: map[string]map[string]string{},
		Choices:    map[string]map[string]map[string]string{},
	}

	header := -1
	for i, row := range rows {
		if len(row) >= 5 && strings.EqualFold(strings.TrimSpace(row[0]), "type") {
			header = i
			break
		}
	}
	if header < 0 {
		return Profile{}, fmt.Errorf("choice-detail header row not found")
	}

	lastIO := ""
	for _, row := range rows[header+1:] {
		if isBlankRow(row) {
			continue
		}
		ioName := cellAt(row, 0)
		if ioName != "" {
			switch strings.ToLower(ioName) {
			case "input":
				lastIO = "I"
			case "output":
				lastIO = "O"
			default:
				continue
			}
		}
		if lastIO == "" {
			continue
		}
		catID := normalizeID(cellAt(row, 1))
		choiceID := normalizeID(cellAt(row, 2))
		category := cellAt(row, 3)
		detail := cellAt(row, 4)
		if catID == "" || choiceID == "" || category == "" {
			continue
		}

		if _, ok := profile.Categories[lastIO]; !ok {
			profile.Categories[lastIO] = map[string]string{}
		}
		profile.Categories[lastIO][catID] = category

		if _, ok := profile.Choices[lastIO]; !ok {
			profile.Choices[lastIO] = map[string]map[string]string{}
		}
		if _, ok := profile.Choices[lastIO][catID]; !ok {
			profile.Choices[lastIO][catID] = map[string]string{}
		}
		profile.Choices[lastIO][catID][choiceID] = detail
	}
	return profile, nil
}

func framesFromRows(rows [][]string) ([]IOCTF, error) {
	start := -1
	for i, row := range rows {
		if len(row) > 0 && strings.EqualFold(strings.TrimSpace(row[0]), "[START]") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		start = 0
	}

	frames := []IOCTF{}
	for _, row := range rows[start:] {
		if isBlankRow(row) || len(row) < 2 {
			continue
		}
		id := normalizeID(cellAt(row, 0))
		if id == "" {
			continue
		}
		choices := ChoiceList{}
		for _, raw := range row[1:] {
			raw = normalizeChoice(raw)
			if raw == "" {
				continue
			}
			choices = append(choices, raw)
		}
		if len(choices) == 0 {
			continue
		}
		frames = append(frames, IOCTF{ID: id, Choices: choices})
	}
	if err := validateFrames(frames); err != nil {
		return nil, err
	}
	return frames, nil
}

func normalizeChoice(raw string) string {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) != 3 {
		return strings.TrimSpace(raw)
	}
	parts[0] = strings.ToUpper(strings.TrimSpace(parts[0]))
	parts[1] = normalizeID(parts[1])
	parts[2] = normalizeID(parts[2])
	return strings.Join(parts, "-")
}

func normalizeID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil && f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return raw
}

func cellAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func isBlankRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
