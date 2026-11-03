package config

import "testing"

func TestParseContainerRef(t *testing.T) {
	cases := []struct{ in, account, container string }{
		{"123", "", "123"},
		{" 123 ", "", "123"},
		{"gtm-abc12", "", "GTM-ABC12"},
		{"accounts/1/containers/2", "1", "2"},
		{"accounts/1/containers/2/workspaces/3", "1", "2"},
		{"https://tagmanager.google.com/#/container/accounts/1/containers/2/workspaces/3", "1", "2"},
	}
	for _, c := range cases {
		a, ct := ParseContainerRef(c.in)
		if a != c.account || ct != c.container {
			t.Errorf("ParseContainerRef(%q) = (%q, %q), want (%q, %q)", c.in, a, ct, c.account, c.container)
		}
	}
}

func TestResolveWorkspaceBelongsToSavedContainer(t *testing.T) {
	t.Setenv(envConfigDir, t.TempDir())
	for _, k := range []string{envAccountID, envContainerID, envWorkspaceID, envClientID, envClientSecret} {
		t.Setenv(k, "")
	}
	if err := SaveCredentials(&Credentials{AccountID: "1", ContainerID: "2", WorkspaceID: "3"}); err != nil {
		t.Fatal(err)
	}

	r, err := Resolve(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if r.AccountID != "1" || r.Container != "2" || r.WorkspaceID != "3" {
		t.Errorf("saved defaults not used: %+v", r)
	}

	// 別のコンテナを指したら、保存済みのワークスペースもアカウントも持ち込まない。
	r, err = Resolve(Overrides{Container: "9"})
	if err != nil {
		t.Fatal(err)
	}
	if r.WorkspaceID != "" || r.AccountID != "" {
		t.Errorf("saved workspace/account leaked into another container: %+v", r)
	}

	r, err = Resolve(Overrides{Container: "accounts/5/containers/9", Workspace: "7"})
	if err != nil {
		t.Fatal(err)
	}
	if r.AccountID != "5" || r.Container != "9" || r.WorkspaceID != "7" {
		t.Errorf("path override not applied: %+v", r)
	}
}

func TestRequireForAPI(t *testing.T) {
	full := &Resolved{ClientID: "id", ClientSecret: "secret", RefreshToken: "rt"}
	if err := full.RequireForAPI(); err != nil {
		t.Errorf("RequireForAPI() with full creds returned error: %v", err)
	}
	if err := (&Resolved{}).RequireForAPI(); err == nil {
		t.Error("RequireForAPI() with empty creds: expected error, got nil")
	}
}
