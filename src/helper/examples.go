package helper

func FastJSONExampleFrames() []IOCTF {
	return []IOCTF{
		{
			ID: "normal-object-preserved",
			Choices: ChoiceList{
				"I-1-1", "I-2-1", "I-3-1", "I-4-2", "I-5-1", "I-6-1", "I-7-1", "I-8-1", "I-9-1",
				"O-1-1", "O-2-1", "O-3-1", "O-4-1", "O-5-1", "O-6-1",
			},
		},
		{
			ID: "single-quote-preserved",
			Choices: ChoiceList{
				"I-1-1", "I-2-1", "I-3-1", "I-4-2", "I-5-2", "I-6-1", "I-7-2", "I-8-3", "I-9-2",
				"O-1-1", "O-2-1", "O-3-1", "O-4-1", "O-5-1", "O-6-1",
			},
		},
		{
			ID: "overflow-numeric-defaults",
			Choices: ChoiceList{
				"I-1-1", "I-2-2", "I-3-2", "I-4-4", "I-5-3", "I-6-2", "I-7-1", "I-8-1", "I-9-1",
				"O-1-1", "O-2-2", "O-3-2", "O-4-2", "O-5-2", "O-6-2",
			},
		},
		{
			ID: "invalid-json-exception",
			Choices: ChoiceList{
				"I-1-4", "I-2-4", "I-3-4", "I-4-5", "I-5-4", "I-6-3", "I-7-1", "I-8-1", "I-9-1",
				"O-1-2", "O-2-3", "O-3-3", "O-4-3", "O-5-3", "O-6-3",
			},
		},
	}
}

type IdentifiedMR struct {
	ID             int    `json:"id"`
	SourceFrameID  string `json:"source_frame_id"`
	FollowFrameID  string `json:"follow_frame_id"`
	SourceCTF      string `json:"source_ctf"`
	FollowCTF      string `json:"follow_ctf"`
	RelationKind   string `json:"relation_kind"`
	OutputRelation string `json:"output_relation"`
	InputRelation  string `json:"input_relation"`
}

type IdentificationResult struct {
	Frames    []IOCTF        `json:"frames"`
	Relations []IdentifiedMR `json:"relations"`
}
