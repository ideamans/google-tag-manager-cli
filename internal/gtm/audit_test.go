package gtm

import (
	"strings"
	"testing"

	tm "google.golang.org/api/tagmanager/v2"
)

func TestRunAudit(t *testing.T) {
	html := []*tm.Parameter{{Type: "template", Key: "html", Value: "<script>document.write('x')</script>"}}
	v := &tm.ContainerVersion{
		Container: &tm.Container{PublicId: "GTM-TEST"},
		Tag: []*tm.Tag{
			{TagId: "1", Name: "GA4", Type: "gaawe", FiringTriggerId: []string{AllPagesTriggerID},
				Parameter: []*tm.Parameter{{Type: "template", Key: "eventName", Value: "{{Event Name}}"}}},
			{TagId: "2", Name: "Old UA", Type: "ua", FiringTriggerId: []string{"10"}},
			{TagId: "3", Name: "Paused", Type: "html", Paused: true, Parameter: html},
			{TagId: "4", Name: "Orphan", Type: "html", Parameter: html},
			{TagId: "5", Name: "Setup", Type: "html"},
			{TagId: "6", Name: "Uses setup", Type: "img", FiringTriggerId: []string{"12"},
				SetupTag: []*tm.SetupTag{{TagName: "Setup"}}},
			{TagId: "7", Name: "Broken", Type: "html", FiringTriggerId: []string{"10"},
				Parameter: []*tm.Parameter{{Type: "template", Key: "html", Value: "<script>{{Nope}} {{_event}} {{Page URL}}</script>"}}},
		},
		Trigger: []*tm.Trigger{
			{TriggerId: "10", Name: "Used by tag", Type: "pageview",
				Filter: []*tm.Condition{{Type: "contains", Parameter: []*tm.Parameter{{Key: "arg0", Value: "{{Page URL}}"}}}}},
			{TriggerId: "11", Name: "Unused", Type: "click"},
			{TriggerId: "12", Name: "Group", Type: "triggerGroup",
				Parameter: []*tm.Parameter{{Type: "list", Key: "triggerIds", List: []*tm.Parameter{{Type: "triggerReference", Value: "13"}}}}},
			{TriggerId: "13", Name: "In group", Type: "click"},
		},
		BuiltInVariable: []*tm.BuiltInVariable{{Type: "pageUrl", Name: "Page URL"}},
		Variable: []*tm.Variable{
			{VariableId: "20", Name: "Event Name", Type: "v"},
			{VariableId: "21", Name: "Lonely <var>", Type: "jsm",
				Parameter: []*tm.Parameter{{Key: "javascript", Value: "function(){return {{Lonely <var>}}}"}}},
			{VariableId: "22", Name: "Measurement ID", Type: "c"},
		},
		// Google タグ設定からしか参照されない変数は未使用ではない。
		GtagConfig: []*tm.GtagConfig{{GtagConfigId: "30", Type: "googtag",
			Parameter: []*tm.Parameter{{Type: "template", Key: "tagId", Value: "{{Measurement ID}}"}}}},
	}

	a := RunAudit(v, "test")

	want := map[string]map[string]bool{
		CodePausedTag:       {"3": true},
		CodeNoFiringTrigger: {"4": true, "5": false}, // 5 は setup タグなので対象外
		CodeUniversalAnalyt: {"2": true},
		CodeUnusedTrigger:   {"11": true, "10": false, "13": false},
		CodeUnusedVariable:  {"21": true, "20": false, "22": false},
		CodeDuplicateHTML:   {"3": true, "4": true},
		CodeDocumentWrite:   {"3": true},
		CodeBrokenVarRef:    {"7": true, "1": false, "21": false},
	}
	got := map[string]map[string]bool{}
	for _, f := range a.Findings {
		if got[f.Code] == nil {
			got[f.Code] = map[string]bool{}
		}
		got[f.Code][f.ID] = true
	}
	for code, ids := range want {
		for id, expected := range ids {
			if got[code][id] != expected {
				t.Errorf("%s for %s: got %v, want %v", code, id, got[code][id], expected)
			}
		}
	}
	for _, f := range a.Findings {
		if f.Code == CodeBrokenVarRef && (!strings.Contains(f.Message, "{{Nope}}") || strings.Contains(f.Message, "_event") || strings.Contains(f.Message, "Page URL")) {
			t.Errorf("broken reference message: %s", f.Message)
		}
	}
	if len(a.CustomHTML) != 4 || a.Counts["tags"] != 7 {
		t.Errorf("customHtml=%d tags=%d", len(a.CustomHTML), a.Counts["tags"])
	}
	if a.Container != "GTM-TEST" {
		t.Errorf("container = %q", a.Container)
	}
}
