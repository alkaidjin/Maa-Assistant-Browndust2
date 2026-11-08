package project

import (
	"encoding/json"
	"testing"
)

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBuildOverrideMergesTaskAndSelectedCase(t *testing.T) {
	task := Task{
		Name:  "DailyAll",
		Entry: "StartGame",
		PipelineOverride: decode(t, `{
		  "EnterGame": {"next": ["StartDraw", "DailyAllEnd"]},
		  "StartDraw": {"max_hit": 1}
		}`),
	}
	opts := []Option{{
		Name: "ChooseWood",
		Cases: []OptionCase{
			{Name: "OreHunt", PipelineOverride: decode(t, `{"HuntEntry": {"attach": {"wood": "ore"}}}`)},
			{Name: "WoodHunt", PipelineOverride: decode(t, `{"HuntEntry": {"attach": {"wood": "wood"}}}`)},
		},
	}}

	got := BuildOverride(task, opts, map[string][]string{"ChooseWood": {"OreHunt"}})

	if len(got["EnterGame"].(map[string]any)["next"].([]any)) != 2 {
		t.Fatalf("task override lost: %#v", got["EnterGame"])
	}
	attach := got["HuntEntry"].(map[string]any)["attach"].(map[string]any)
	if attach["wood"] != "ore" {
		t.Fatalf("option case not applied: %#v", attach)
	}
}

func TestBuildOverrideUnselectedOptionIsIgnored(t *testing.T) {
	task := Task{Name: "X", PipelineOverride: decode(t, `{"A": {"v": 1}}`)}
	opts := []Option{{Name: "O", Cases: []OptionCase{{Name: "c1", PipelineOverride: decode(t, `{"B": {"v": 2}}`)}}}}

	got := BuildOverride(task, opts, nil)
	if _, ok := got["B"]; ok {
		t.Fatalf("unselected option leaked into override: %#v", got)
	}
	if _, ok := got["A"]; !ok {
		t.Fatal("task override missing")
	}
}

func TestBuildOverrideEmptyIsNotNil(t *testing.T) {
	got := BuildOverride(Task{}, nil, nil)
	if got == nil {
		t.Fatal("want empty map, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %#v", got)
	}
}

func TestFindOption(t *testing.T) {
	pi := &Interface{Option: []Option{{Name: "ChooseRock"}, {Name: "开启自动战斗"}}}
	if o, ok := pi.FindOption("开启自动战斗"); !ok || o.Name != "开启自动战斗" {
		t.Fatalf("FindOption = %+v, %v", o, ok)
	}
	if _, ok := pi.FindOption("nope"); ok {
		t.Fatal("want miss")
	}
}
