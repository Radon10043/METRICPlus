package helper

import (
	"os"
	"testing"
)

func TestLoadFastJSONSpecification(t *testing.T) {
	specPath := "../../data/fastjson.xlsx"
	if _, err := os.Stat(specPath); err != nil {
		t.Skipf("specification file is not available: %v", err)
	}

	spec, err := LoadSpecification(specPath, DefaultChoiceSheet, DefaultFrameSheet)
	if err != nil {
		t.Fatalf("LoadSpecification() error = %v", err)
	}
	if got, want := len(spec.Frames), 365; got != want {
		t.Fatalf("len(Frames) = %d, want %d", got, want)
	}
	if got := spec.Profile.CategoryName("I", "1"); got != "JsonFormat" {
		t.Fatalf("input category I-1 = %q", got)
	}
	if got := spec.Profile.ChoiceName("O", "1", "2"); got != "JSONException raised" {
		t.Fatalf("output choice O-1-2 = %q", got)
	}
}

func TestLoadFastJSONYAMLSpecification(t *testing.T) {
	specPath := "../../data/fastjson.spec.yaml"
	if _, err := os.Stat(specPath); err != nil {
		t.Skipf("YAML specification file is not available: %v", err)
	}

	spec, err := LoadSpecificationFile(specPath, DefaultChoiceSheet, DefaultFrameSheet)
	if err != nil {
		t.Fatalf("LoadSpecificationFile() error = %v", err)
	}
	if got, want := len(spec.Frames), 365; got != want {
		t.Fatalf("len(Frames) = %d, want %d", got, want)
	}
	if got := spec.Profile.ChoiceName("I", "5", "2"); got != "Include escape characters, such as \\\\" {
		t.Fatalf("input choice I-5-2 = %q", got)
	}
}
