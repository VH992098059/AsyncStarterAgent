package synthesis

import (
	"testing"
)

func TestExtractMarks(t *testing.T) {
	md := "Hello [待补充:具体数字] world [待补充:负责人]"
	marks := ExtractMarks(md)
	if len(marks) != 2 {
		t.Fatalf("expected 2 marks, got %d", len(marks))
	}
	if marks[0].Hint != "具体数字" {
		t.Errorf("first hint: %s", marks[0].Hint)
	}
	if marks[1].Hint != "负责人" {
		t.Errorf("second hint: %s", marks[1].Hint)
	}
	if marks[0].ID != "mark-0" {
		t.Errorf("first id: %s", marks[0].ID)
	}
	if marks[1].ID != "mark-1" {
		t.Errorf("second id: %s", marks[1].ID)
	}
}

func TestReplaceMark(t *testing.T) {
	md := "Hello [待补充:数字] world"
	marks := ExtractMarks(md)
	out, err := ReplaceMark(md, marks[0].ID, marks, "42")
	if err != nil {
		t.Fatal(err)
	}
	if out != "Hello 42 world" {
		t.Errorf("got: %s", out)
	}
}

func TestReplaceMark_NotFound(t *testing.T) {
	md := "Hello [待补充:数字] world"
	marks := ExtractMarks(md)
	_, err := ReplaceMark(md, "non-existent", marks, "42")
	if err == nil {
		t.Fatal("expected error for missing mark")
	}
}

func TestReplaceMark_DuplicateHints(t *testing.T) {
	md := "[待补充:数字] + [待补充:数字] = ?"
	marks := ExtractMarks(md)
	if len(marks) != 2 {
		t.Fatalf("expected 2 marks, got %d", len(marks))
	}
	out, err := ReplaceMark(md, marks[1].ID, marks, "two")
	if err != nil {
		t.Fatal(err)
	}
	if out != "[待补充:数字] + two = ?" {
		t.Errorf("got: %s", out)
	}
}

func TestReplaceMark_SpecialValue(t *testing.T) {
	md := "Price: [待补充:price] USD"
	marks := ExtractMarks(md)
	out, err := ReplaceMark(md, marks[0].ID, marks, "$100")
	if err != nil {
		t.Fatal(err)
	}
	if out != "Price: $100 USD" {
		t.Errorf("got: %s", out)
	}
}

func TestReplaceMark_AfterExtractNoMarks(t *testing.T) {
	md := "Hello [待补充:数字] world"
	marks := ExtractMarks(md)
	out, err := ReplaceMark(md, marks[0].ID, marks, "42")
	if err != nil {
		t.Fatal(err)
	}
	if len(ExtractMarks(out)) != 0 {
		t.Errorf("expected 0 marks after replacement, got %d", len(ExtractMarks(out)))
	}
}

func TestCompleteness(t *testing.T) {
	cases := []struct {
		md  string
		min float32
	}{
		{"no marks here", 0.9},
		{"one [待补充:foo] mark", 0.1},
		{"[待补充:a][待补充:b][待补充:c][待补充:d][待补充:e][待补充:f]", 0.0},
	}
	for _, c := range cases {
		got := Completeness(c.md)
		if got < c.min-0.05 {
			t.Errorf("md=%q: completeness=%v < min %v", c.md, got, c.min)
		}
	}
}
