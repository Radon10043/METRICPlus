package helper

import (
	"fmt"
	"sort"
)

type OutputGroup struct {
	Key    string  `json:"key"`
	Frames []IOCTF `json:"frames"`
}

type CandidatePair struct {
	SourceFrame   IOCTF  `json:"source_frame"`
	FollowUpFrame IOCTF  `json:"followup_frame"`
	RelationKind  string `json:"relation_kind"`
	OutputSummary string `json:"output_summary"`
}

func GroupByOutputFrame(frames []IOCTF) []OutputGroup {
	groupMap := map[string][]IOCTF{}
	for _, frame := range frames {
		groupMap[frame.OutputKey()] = append(groupMap[frame.OutputKey()], frame)
	}
	keys := make([]string, 0, len(groupMap))
	for key := range groupMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	groups := make([]OutputGroup, 0, len(keys))
	for _, key := range keys {
		groups = append(groups, OutputGroup{Key: key, Frames: groupMap[key]})
	}
	return groups
}

func CandidatePairs(frames []IOCTF, mode string, limit int) []CandidatePair {
	groups := GroupByOutputFrame(frames)
	out := []CandidatePair{}
	add := func(a, b IOCTF, kind string) bool {
		summary, ok := outputRelationSummary(a.Choices.Output(), b.Choices.Output(), kind)
		if !ok {
			return true
		}
		out = append(out, CandidatePair{
			SourceFrame:   a,
			FollowUpFrame: b,
			RelationKind:  kind,
			OutputSummary: summary,
		})
		return limit <= 0 || len(out) < limit
	}

	if mode == "" || mode == "within" || mode == "all" {
		for _, group := range groups {
			for i := 0; i < len(group.Frames); i++ {
				for j := 0; j < len(group.Frames); j++ {
					if i == j {
						continue
					}
					if !add(group.Frames[i], group.Frames[j], "within-output-group") {
						return out
					}
				}
			}
		}
	}

	if mode == "" || mode == "across" || mode == "all" {
		for i := 0; i < len(groups); i++ {
			for j := 0; j < len(groups); j++ {
				if i == j {
					continue
				}
				for _, a := range groups[i].Frames {
					for _, b := range groups[j].Frames {
						if !add(a, b, "across-output-groups") {
							return out
						}
					}
				}
			}
		}
	}
	return out
}

func outputRelationSummary(source, follow ChoiceList, kind string) (string, bool) {
	if len(source) == 0 || len(follow) == 0 {
		return "", false
	}
	if source.OutputKey() == follow.OutputKey() {
		return "Ro: output test frame remains unchanged", true
	}
	return fmt.Sprintf("Ro: output test frame changes from {%s} to {%s}", source.String(), follow.String()), true
}
