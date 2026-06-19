package helper

import "fmt"

type Profile struct {
	Categories map[string]map[string]string            `json:"categories" yaml:"categories"`
	Choices    map[string]map[string]map[string]string `json:"choices" yaml:"choices"`
}

func FastJSONProfile() Profile {
	return Profile{
		Categories: map[string]map[string]string{
			"I": {
				"1": "JsonFormat",
				"2": "Float",
				"3": "Long",
				"4": "Date",
				"5": "String",
				"6": "Enum",
				"7": "ClassType",
				"8": "Feature",
				"9": "Quotation",
			},
			"O": {
				"1": "Exception",
				"2": "Float",
				"3": "Long",
				"4": "Date",
				"5": "String",
				"6": "Enum",
			},
		},
		Choices: map[string]map[string]map[string]string{
			"I": {
				"1": {
					"1": "Object",
					"2": "Array",
					"3": "JSON with deleted symbol",
					"4": "Invalid JSON",
				},
				"2": {
					"1": "Normal",
					"2": "Overflow",
					"3": "Null",
					"4": "Does not exist",
				},
				"3": {
					"1": "Normal",
					"2": "Overflow",
					"3": "Null",
					"4": "Does not exist",
				},
				"4": {
					"1": "yyyy-MM-dd",
					"2": "yyyy-MM-ddTHH:mm:ss",
					"3": "yyyy-MM-ddTHH:mm:ss.SSS",
					"4": "Null",
					"5": "Does not exist",
				},
				"5": {
					"1": "String without escape characters",
					"2": "String with escape characters",
					"3": "Null",
					"4": "Does not exist",
				},
				"6": {
					"1": "Existing enum data",
					"2": "Non-existing enum data",
					"3": "Does not exist",
				},
				"7": {
					"1": "JavaBean class",
					"2": "TypeReference",
				},
				"8": {
					"1": "NoFeature",
					"2": "AllowUnQuotedFieldNames",
					"3": "AllowSingleQuotes",
					"4": "AllowISO8601DateFormat",
					"5": "InitStringFieldAsEmpty",
				},
				"9": {
					"1": "DoubleQuote",
					"2": "SingleQuote",
					"3": "NoQuote",
				},
			},
			"O": {
				"1": {
					"1": "No exception raised",
					"2": "JSONException raised",
				},
				"2": {
					"1": "Error less than 1e-6 compared with input",
					"2": "0",
					"3": "Not applicable",
				},
				"3": {
					"1": "Equal to input",
					"2": "0",
					"3": "Not applicable",
				},
				"4": {
					"1": "Equal to input",
					"2": "Null",
					"3": "Not applicable",
				},
				"5": {
					"1": "Equal to input",
					"2": "Null",
					"3": "Not applicable",
				},
				"6": {
					"1": "Equal to input",
					"2": "Null",
					"3": "Not applicable",
				},
			},
		},
	}
}

func (p Profile) CategoryName(io, cat string) string {
	if cats, ok := p.Categories[io]; ok {
		if name, ok := cats[cat]; ok {
			return name
		}
	}
	return fmt.Sprintf("%s-%s", io, cat)
}

func (p Profile) ChoiceName(io, cat, cho string) string {
	if cats, ok := p.Choices[io]; ok {
		if choices, ok := cats[cat]; ok {
			if name, ok := choices[cho]; ok {
				return name
			}
		}
	}
	switch cho {
	case "*":
		return "Any choice except NA"
	case "NA":
		return "Not applicable"
	}
	return fmt.Sprintf("%s-%s-%s", io, cat, cho)
}

func (p Profile) Describe(raw string) string {
	ref, err := ParseChoiceRef(raw)
	if err != nil {
		return raw
	}
	return fmt.Sprintf("%s / %s / %s", ref.IO, p.CategoryName(ref.IO, ref.Category), p.ChoiceName(ref.IO, ref.Category, ref.Choice))
}

func (p Profile) ChoiceColumns(raw string) [3]string {
	ref, err := ParseChoiceRef(raw)
	if err != nil {
		return [3]string{"?", raw, raw}
	}
	return [3]string{ref.IO, p.CategoryName(ref.IO, ref.Category), p.ChoiceName(ref.IO, ref.Category, ref.Choice)}
}
