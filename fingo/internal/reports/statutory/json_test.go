package statutory

import (
	"encoding/json"
	"strings"
	"testing"
)

// The filing document is the shape the schema prescribes.
func TestJSON_ConformsToTheSchema(t *testing.T) {
	document, err := newFixture(t).generate(t).JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	if err := ValidateJSON(document); err != nil {
		t.Fatalf("the filing does not conform to its own schema: %v", err)
	}

	var held map[string]any
	if err := json.Unmarshal(document, &held); err != nil {
		t.Fatalf("the filing is not JSON: %v", err)
	}

	for _, key := range []string{"period", "generated_at", "statements", "warnings"} {
		if _, present := held[key]; !present {
			t.Errorf("the filing has no %q", key)
		}
	}
}

// The validator is not a rubber stamp: a document that breaks any of the rules
// the schema states is refused.
func TestValidateJSON_RefusesWhatTheSchemaForbids(t *testing.T) {
	base, err := newFixture(t).generate(t).JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var document map[string]any
	if err := json.Unmarshal(base, &document); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	cases := map[string]func(map[string]any){
		"a line with no legal basis": func(d map[string]any) {
			firstLine(d)["reference"] = ""
		},
		"a line the basis has not resolved": func(d map[string]any) {
			firstLine(d)["confidence"] = "?"
		},
		"an amount that is not a decimal string": func(d map[string]any) {
			firstLine(d)["amount"] = 42
		},
		"a property the schema does not describe": func(d map[string]any) {
			firstLine(d)["extra"] = "something"
		},
		"a regime the form sets do not cover": func(d map[string]any) {
			d["period"].(map[string]any)["regime"] = "IFRS"
		},
		"no statements at all": func(d map[string]any) {
			d["statements"] = []any{}
		},
		"a statement with no authority": func(d map[string]any) {
			firstStatement(d)["authority"] = ""
		},
		"a warning with nothing to say": func(d map[string]any) {
			d["warnings"] = []any{map[string]any{"statement": "01/GTGT", "item": "x", "reason": ""}}
		},
		"a document that is not an object": func(map[string]any) {},
	}

	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			// A fresh copy each time: the spoilers edit in place.
			var held map[string]any
			if err := json.Unmarshal(base, &held); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			broken, err := json.Marshal(spoil2(held, spoil))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if name == "a document that is not an object" {
				broken = []byte(`"not an object"`)
			}

			if err := ValidateJSON(broken); err == nil {
				t.Errorf("the validator accepted a document that breaks the schema")
			}
		})
	}
}

// spoil2 applies a spoiler and hands the document back.
func spoil2(document map[string]any, spoil func(map[string]any)) map[string]any {
	spoil(document)

	return document
}

func firstStatement(document map[string]any) map[string]any {
	statements, _ := document["statements"].([]any)
	if len(statements) == 0 {
		return map[string]any{}
	}

	held, _ := statements[0].(map[string]any)
	if held == nil {
		held = map[string]any{}
		statements[0] = held
	}

	return held
}

func firstLine(document map[string]any) map[string]any {
	held := firstStatement(document)

	lines, _ := held["lines"].([]any)
	if len(lines) == 0 {
		line := map[string]any{}
		held["lines"] = []any{line}

		return line
	}

	line, _ := lines[0].(map[string]any)
	if line == nil {
		line = map[string]any{}
		lines[0] = line
	}

	return line
}

// The filing copy keeps the names as the forms write them: it is the PDF that
// transliterates, and only because a standard PDF font cannot carry them.
func TestJSON_KeepsTheNamesAsTheFormsWriteThem(t *testing.T) {
	document, err := newFixture(t).generate(t).JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	for _, written := range []string{
		"Thuế GTGT còn được khấu trừ kỳ trước chuyển sang",
		"Quyết toán thuế thu nhập doanh nghiệp",
		"Bảng cân đối kế toán",
	} {
		if !strings.Contains(string(document), written) {
			t.Errorf("the filing does not carry %q as written", written)
		}
	}
}

// The Go-level check is the same rule the schema states, reached without
// rendering a document.
func TestOutput_ValidateRefusesALineWithoutABasis(t *testing.T) {
	out := newFixture(t).generate(t)

	broken := out
	broken.Statements = append([]Statement{}, out.Statements...)
	broken.Statements[0].Lines = append([]LineItem{}, out.Statements[0].Lines...)
	broken.Statements[0].Lines[0].Reference = ""

	if err := broken.Validate(); err == nil {
		t.Fatal("a line with no legal basis passed validation")
	}

	unresolved := out
	unresolved.Statements = append([]Statement{}, out.Statements...)
	unresolved.Statements[0].Lines = append([]LineItem{}, out.Statements[0].Lines...)
	unresolved.Statements[0].Lines[0].Confidence = Unresolved

	if err := unresolved.Validate(); err == nil {
		t.Fatal("an unresolved line passed validation")
	}
}
