package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ideamans/go-llm-cli-kit/llmcmd"

	"github.com/ideamans/google-tag-manager-cli/internal/llmdocs"
)

func TestEmbeddedReference(t *testing.T) {
	g, err := llmdocs.Docs().Markdown()
	if err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	for _, want := range []string{
		"Guide for LLMs",    // 00-guide.md
		"# Command catalog", // 90-commands.md（生成）
		"gtmc tag create",
		"gtmc version publish",
		"gtmc audit",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("埋め込みリファレンスに %q がありません", want)
		}
	}
}

func TestChapterOrder(t *testing.T) {
	sections, err := llmdocs.Docs().Sections()
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	var files []string
	for _, s := range sections {
		files = append(files, s.File)
	}
	if got, want := strings.Join(files, ","), "00-guide.md,90-commands.md"; got != want {
		t.Errorf("chapters = %s, want %s", got, want)
	}
}

// ガイドが説明している audit のコードが実装と食い違っていないこと。
func TestGuideListsAuditCodes(t *testing.T) {
	g, _ := llmdocs.Docs().Markdown()
	for _, code := range []string{
		"PAUSED_TAG", "TAG_WITHOUT_FIRING_TRIGGER", "UNUSED_TRIGGER", "UNUSED_VARIABLE",
		"UNIVERSAL_ANALYTICS", "DUPLICATE_CUSTOM_HTML", "DOCUMENT_WRITE", "DUPLICATE_TAG_NAME",
		"BROKEN_VARIABLE_REFERENCE",
	} {
		if !strings.Contains(g, code) {
			t.Errorf("00-guide.md does not explain audit code %s", code)
		}
	}
}

func TestLegacyLLMFlag(t *testing.T) {
	var out bytes.Buffer
	handled, err := llmcmd.HandleLegacy([]string{"tag", "--llm"}, LLMConfig(), &out)
	if err != nil || !handled {
		t.Fatalf("HandleLegacy: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(out.String(), "Guide for LLMs") {
		t.Error("--llm output is not the reference")
	}
}

// 取り消しにくい操作は --yes なしでは API を呼ぶ前に止まること。
func TestDestructiveCommandsRequireYes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GTM_HOME", dir)
	bulk := filepath.Join(dir, "bulk.json")
	if err := os.WriteFile(bulk, []byte(`{"changes":[{"tag":{"tagId":"7"},"changeStatus":"deleted"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"tag", "delete", "1"},
		{"trigger", "delete", "1"},
		{"variable", "delete", "1"},
		{"client", "delete", "1"},
		{"transformation", "delete", "1"},
		{"zone", "delete", "1"},
		{"gtag-config", "delete", "1"},
		{"workspace", "delete", "1"},
		{"workspace", "bulk-update", "--file", bulk},
		{"version", "publish", "3"},
		{"version", "delete", "3"},
		{"environment", "delete", "5"},
		{"environment", "reauthorize", "5"},
	} {
		root := newRootCmd()
		root.SetArgs(args)
		root.SetOut(&bytes.Buffer{})
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "--yes") {
			t.Errorf("%v: expected a --yes error, got %v", args, err)
		}
	}
}

// ギャラリーのテンプレートは権限の承諾なしでは API を呼ぶ前に止まること。
func TestTemplateImportRequiresAcknowledgement(t *testing.T) {
	t.Setenv("GTM_HOME", t.TempDir())
	root := newRootCmd()
	root.SetArgs([]string{"template", "import", "--owner", "o", "--repository", "r"})
	root.SetOut(&bytes.Buffer{})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "--acknowledge-permissions") {
		t.Errorf("expected an acknowledgement error, got %v", err)
	}
}
