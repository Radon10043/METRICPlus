package helper

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type ChoiceRef struct {
	IO       string `json:"io" yaml:"io"`
	Category string `json:"category" yaml:"category"`
	Choice   string `json:"choice" yaml:"choice"`
}

func ParseChoiceRef(raw string) (ChoiceRef, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, "-")
	if len(parts) != 3 {
		return ChoiceRef{}, fmt.Errorf("choice %q must use I-cat-choice or O-cat-choice format", raw)
	}
	io := strings.ToUpper(strings.TrimSpace(parts[0]))
	if io != "I" && io != "O" {
		return ChoiceRef{}, fmt.Errorf("choice %q has invalid IO prefix %q", raw, parts[0])
	}
	cat := strings.TrimSpace(parts[1])
	cho := strings.TrimSpace(parts[2])
	if cat == "" || cho == "" {
		return ChoiceRef{}, fmt.Errorf("choice %q has empty category or choice", raw)
	}
	return ChoiceRef{IO: io, Category: cat, Choice: cho}, nil
}

func (r ChoiceRef) String() string {
	return r.IO + "-" + r.Category + "-" + r.Choice
}

type ChoiceList []string

func (c *ChoiceList) UnmarshalJSON(data []byte) error {
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*c = NormalizeChoices(arr)
		return nil
	}

	var csv string
	if err := json.Unmarshal(data, &csv); err != nil {
		return err
	}
	*c = NormalizeChoices(SplitChoices(csv))
	return nil
}

func (c ChoiceList) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(c))
}

func (c *ChoiceList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*c = NormalizeChoices(SplitChoices(value.Value))
		return nil
	case yaml.SequenceNode:
		vals := make([]string, 0, len(value.Content))
		for _, node := range value.Content {
			vals = append(vals, node.Value)
		}
		*c = NormalizeChoices(vals)
		return nil
	default:
		return fmt.Errorf("choices must be a comma-separated string or a list")
	}
}

func (c ChoiceList) MarshalYAML() (any, error) {
	return c.String(), nil
}

func SplitChoices(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func NormalizeChoices(in []string) ChoiceList {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return ChoiceList(out)
}

func (c ChoiceList) String() string {
	return strings.Join(c, ",")
}

func (c ChoiceList) Refs() ([]ChoiceRef, error) {
	refs := make([]ChoiceRef, 0, len(c))
	for _, raw := range c {
		ref, err := ParseChoiceRef(raw)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func (c ChoiceList) Input() ChoiceList {
	return filterChoices(c, "I")
}

func (c ChoiceList) Output() ChoiceList {
	return filterChoices(c, "O")
}

func (c ChoiceList) OutputKey() string {
	out := append([]string(nil), c.Output()...)
	sort.Strings(out)
	return strings.Join(out, ",")
}

func filterChoices(in ChoiceList, io string) ChoiceList {
	out := make([]string, 0, len(in))
	prefix := io + "-"
	for _, raw := range in {
		if strings.HasPrefix(strings.ToUpper(raw), prefix) {
			out = append(out, raw)
		}
	}
	return ChoiceList(out)
}

type IOCTF struct {
	ID      string     `json:"id,omitempty" yaml:"id,omitempty"`
	Choices ChoiceList `json:"choices" yaml:"choices"`
}

func (f IOCTF) Name() string {
	if f.ID != "" {
		return f.ID
	}
	return f.Choices.String()
}

func (f IOCTF) InputKey() string {
	return f.Choices.Input().String()
}

func (f IOCTF) OutputKey() string {
	return f.Choices.OutputKey()
}

func (f IOCTF) Validate() error {
	seen := map[string]bool{}
	for _, raw := range f.Choices {
		ref, err := ParseChoiceRef(raw)
		if err != nil {
			return err
		}
		key := ref.IO + "-" + ref.Category
		if seen[key] {
			return fmt.Errorf("frame %q contains more than one choice for %s", f.Name(), key)
		}
		seen[key] = true
	}
	return nil
}

type FrameSet struct {
	Frames []IOCTF `json:"frames" yaml:"frames"`
}

func DecodeFrames(data []byte) ([]IOCTF, error) {
	var wrapped FrameSet
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Frames) > 0 {
		return wrapped.Frames, validateFrames(wrapped.Frames)
	}
	var frames []IOCTF
	if err := json.Unmarshal(data, &frames); err != nil {
		return nil, err
	}
	return frames, validateFrames(frames)
}

func validateFrames(frames []IOCTF) error {
	seen := map[string]bool{}
	for _, f := range frames {
		if err := f.Validate(); err != nil {
			return err
		}
		key := f.Choices.String()
		if seen[key] {
			return fmt.Errorf("duplicate IO-CTF: %s", key)
		}
		seen[key] = true
	}
	return nil
}
