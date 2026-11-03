package gtm

import (
	"encoding/json"
	"testing"
)

func TestStripIdentity(t *testing.T) {
	in := `{"path":"accounts/1/containers/2/workspaces/3/tags/4","accountId":"1","containerId":"2",
	  "workspaceId":"3","tagId":"4","fingerprint":"9","tagManagerUrl":"u","name":"T","firingTriggerId":["5"]}`
	out, err := stripIdentity([]byte(in), "tagId")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	for _, k := range []string{"path", "accountId", "containerId", "workspaceId", "tagId", "fingerprint", "tagManagerUrl"} {
		if _, ok := m[k]; ok {
			t.Errorf("%s was kept", k)
		}
	}
	if m["name"] != "T" || m["firingTriggerId"] == nil {
		t.Errorf("content was lost: %v", m)
	}
}

func TestKindsAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Kinds {
		if k.List == nil || k.Get == nil || k.Create == nil || k.Update == nil || k.Delete == nil {
			t.Errorf("%s: missing operation", k.Name)
		}
		if k.ExportKey == "" || k.IDField == "" || seen[k.Name] {
			t.Errorf("%s: bad metadata", k.Name)
		}
		seen[k.Name] = true
	}
	if GtagConfigKind.Revert != nil {
		t.Error("gtag config has no revert in the API")
	}
}
