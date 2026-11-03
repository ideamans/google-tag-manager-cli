package cmd

import (
	"testing"

	tm "google.golang.org/api/tagmanager/v2"
)

func TestParseProposedChange(t *testing.T) {
	obj := `{"changes":[{"folder":{"folderId":"new_1","name":"F"},"changeStatus":"added"},
	  {"tag":{"tagId":"new_2","name":"T","type":"html","parentFolderId":"new_1"},"changeStatus":"added"}]}`
	pc, err := parseProposedChange([]byte(obj))
	if err != nil || len(pc.Changes) != 2 || pc.Changes[1].Tag.ParentFolderId != "new_1" {
		t.Fatalf("object form: %v %+v", err, pc)
	}
	pc, err = parseProposedChange([]byte(` [{"trigger":{"triggerId":"new_1"},"changeStatus":"added"}]`))
	if err != nil || len(pc.Changes) != 1 {
		t.Fatalf("array form: %v", err)
	}
	if _, err := parseProposedChange([]byte(`{"changes":[{"tagg":{}}]}`)); err == nil {
		t.Error("unknown field was accepted")
	}
	if _, err := parseProposedChange([]byte(`{"changes":[]}`)); err == nil {
		t.Error("empty change list was accepted")
	}
}

func TestEntityFingerprint(t *testing.T) {
	e := &tm.Entity{Trigger: &tm.Trigger{TriggerId: "3", Fingerprint: "123"}, ChangeStatus: "updated"}
	if got := entityFingerprint(e); got != "123" {
		t.Errorf("fingerprint = %q", got)
	}
	if entityFingerprint(nil) != "" {
		t.Error("nil entity")
	}
}

func TestViewEnvAddsPreviewQuery(t *testing.T) {
	m := viewEnv(&tm.Environment{EnvironmentId: "21", AuthorizationCode: "abc", Name: "stg"})
	if m["previewQuery"] != "gtm_auth=abc&gtm_preview=env-21&gtm_cookies_win=x" || m["name"] != "stg" {
		t.Errorf("got %v", m)
	}
}
