package gtm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	tm "google.golang.org/api/tagmanager/v2"
)

// AllPagesTriggerID は組み込みトリガー「All Pages（すべてのページ）」のID。
// トリガー一覧には現れず、タグの firingTriggerId にだけ出てくる。
const AllPagesTriggerID = "2147479553"

// builtInTriggerMin より大きいトリガーIDは組み込みトリガー（All Pages など）。
const builtInTriggerMin = 2147479000

// tagTypeLabels はよく見るタグの type と UI 上の名前の対応。
var tagTypeLabels = map[string]string{
	"html":                "Custom HTML",
	"img":                 "Custom Image",
	"googtag":             "Google Tag",
	"gaawe":               "GA4 Event",
	"gaawc":               "GA4 Configuration (legacy)",
	"ua":                  "Universal Analytics (deprecated)",
	"awct":                "Google Ads Conversion Tracking",
	"sp":                  "Google Ads Remarketing",
	"gclidw":              "Conversion Linker",
	"flc":                 "Floodlight Counter",
	"fls":                 "Floodlight Sales",
	"baut":                "Microsoft Advertising UET",
	"bzi":                 "LinkedIn Insight",
	"hjtc":                "Hotjar Tracking Code",
	"cegg":                "Crazy Egg",
	"pntr":                "Pinterest",
	"twitter_website_tag": "X (Twitter) Website Tag",
}

// Audit はコンテナの棚卸し結果。判断（消すか・まとめるか）は利用者や
// エージェントに委ね、ここでは機械的に確かめられる事実だけを並べる。
type Audit struct {
	Container string         `json:"container"`
	Source    string         `json:"source"`
	Counts    map[string]int `json:"counts"`
	TagTypes  []TypeCount    `json:"tagTypes"`
	Findings  []Finding      `json:"findings"`
	// CustomHTML はカスタム HTML タグを本文の大きい順に並べたもの。
	// 表示速度への影響を見積もる起点になる。
	CustomHTML []HTMLTag `json:"customHtml"`
}

// TypeCount はタグの種類ごとの数。
type TypeCount struct {
	Type   string `json:"type"`
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Paused int    `json:"paused"`
}

// Finding は棚卸しで見つかった事実1件。
type Finding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // warn | info
	Kind     string `json:"kind"`     // tag | trigger | variable
	ID       string `json:"id"`
	Name     string `json:"name"`
	Message  string `json:"message"`
}

// HTMLTag はカスタム HTML タグの要約。
type HTMLTag struct {
	TagID         string   `json:"tagId"`
	Name          string   `json:"name"`
	Bytes         int      `json:"bytes"`
	Paused        bool     `json:"paused"`
	AllPages      bool     `json:"allPages"`
	DocumentWrite bool     `json:"documentWrite"`
	Triggers      []string `json:"firingTriggerId"`
}

// Finding のコード。00-guide.md の説明と揃えること。
const (
	CodePausedTag        = "PAUSED_TAG"
	CodeNoFiringTrigger  = "TAG_WITHOUT_FIRING_TRIGGER"
	CodeUnusedTrigger    = "UNUSED_TRIGGER"
	CodeUnusedVariable   = "UNUSED_VARIABLE"
	CodeUniversalAnalyt  = "UNIVERSAL_ANALYTICS"
	CodeDuplicateHTML    = "DUPLICATE_CUSTOM_HTML"
	CodeDocumentWrite    = "DOCUMENT_WRITE"
	CodeDuplicateTagName = "DUPLICATE_TAG_NAME"
	CodeBrokenVarRef     = "BROKEN_VARIABLE_REFERENCE"
)

// RunAudit は ContainerVersion を棚卸しする。API は呼ばない（純粋な関数）。
func RunAudit(v *tm.ContainerVersion, source string) *Audit {
	a := &Audit{
		Source: source,
		Counts: map[string]int{
			"tags":             len(v.Tag),
			"triggers":         len(v.Trigger),
			"variables":        len(v.Variable),
			"folders":          len(v.Folder),
			"builtInVariables": len(v.BuiltInVariable),
			"customTemplates":  len(v.CustomTemplate),
			"clients":          len(v.Client),
			"transformations":  len(v.Transformation),
			"zones":            len(v.Zone),
			"gtagConfigs":      len(v.GtagConfig),
		},
		Findings:   []Finding{},
		CustomHTML: []HTMLTag{},
	}
	if v.Container != nil {
		a.Container = v.Container.PublicId
	}

	templateNames := map[string]string{} // タグの type -> テンプレート名
	for _, t := range v.CustomTemplate {
		gid := ""
		if t.GalleryReference != nil {
			gid = t.GalleryReference.GalleryTemplateId
		}
		templateNames[TemplateType(t.ContainerId, t.TemplateId, gid)] = t.Name
	}

	// ---- タグ ----
	types := map[string]*TypeCount{}
	usedTriggers := map[string]bool{}
	setupOrTeardown := map[string]bool{}
	for _, t := range v.Tag {
		for _, s := range t.SetupTag {
			setupOrTeardown[s.TagName] = true
		}
		for _, s := range t.TeardownTag {
			setupOrTeardown[s.TagName] = true
		}
	}
	htmlBodies := map[string][]*tm.Tag{}
	names := map[string][]*tm.Tag{}
	for _, t := range v.Tag {
		tc := types[t.Type]
		if tc == nil {
			tc = &TypeCount{Type: t.Type, Label: tagTypeLabel(t.Type, templateNames)}
			types[t.Type] = tc
		}
		tc.Count++
		if t.Paused {
			tc.Paused++
			a.add(CodePausedTag, "info", "tag", t.TagId, t.Name, "paused; it never fires, so it is a deletion candidate")
		}
		for _, id := range t.FiringTriggerId {
			usedTriggers[id] = true
		}
		for _, id := range t.BlockingTriggerId {
			usedTriggers[id] = true
		}
		if len(t.FiringTriggerId) == 0 && !setupOrTeardown[t.Name] && !t.Paused {
			a.add(CodeNoFiringTrigger, "warn", "tag", t.TagId, t.Name, "has no firing trigger and is not a setup/teardown tag of another tag, so it never fires")
		}
		if t.Type == "ua" {
			a.add(CodeUniversalAnalyt, "warn", "tag", t.TagId, t.Name, "Universal Analytics stopped processing data; this tag only costs load time")
		}
		names[t.Name] = append(names[t.Name], t)

		if t.Type == "html" {
			html := paramValue(t.Parameter, "html")
			h := HTMLTag{
				TagID: t.TagId, Name: t.Name, Bytes: len(html), Paused: t.Paused,
				AllPages:      contains(t.FiringTriggerId, AllPagesTriggerID),
				DocumentWrite: strings.Contains(html, "document.write"),
				Triggers:      t.FiringTriggerId,
			}
			a.CustomHTML = append(a.CustomHTML, h)
			if h.DocumentWrite {
				a.add(CodeDocumentWrite, "warn", "tag", t.TagId, t.Name, "custom HTML uses document.write, which blocks rendering")
			}
			key := strings.TrimSpace(html)
			if key != "" {
				htmlBodies[key] = append(htmlBodies[key], t)
			}
		}
	}
	for _, group := range htmlBodies {
		if len(group) < 2 {
			continue
		}
		ids := tagIDs(group)
		for _, t := range group {
			a.add(CodeDuplicateHTML, "warn", "tag", t.TagId, t.Name, "the same custom HTML is also in tag(s) "+strings.Join(without(ids, t.TagId), ", "))
		}
	}
	for name, group := range names {
		if len(group) < 2 {
			continue
		}
		a.add(CodeDuplicateTagName, "info", "tag", strings.Join(tagIDs(group), ","), name, fmt.Sprintf("%d tags share this name", len(group)))
	}

	// ---- トリガー ----
	for _, v := range v.Variable {
		for _, id := range v.EnablingTriggerId {
			usedTriggers[id] = true
		}
		for _, id := range v.DisablingTriggerId {
			usedTriggers[id] = true
		}
	}
	// トリガーグループは他のトリガーを参照する。参照先は使われているとみなす。
	for _, tr := range v.Trigger {
		if tr.Type == "triggerGroup" {
			walkParams(tr.Parameter, func(p *tm.Parameter) {
				if p.Type == "triggerReference" {
					usedTriggers[p.Value] = true
				}
			})
		}
	}
	for _, tr := range v.Trigger {
		if !usedTriggers[tr.TriggerId] {
			a.add(CodeUnusedTrigger, "info", "trigger", tr.TriggerId, tr.Name, "no tag, variable or trigger group refers to it")
		}
	}

	// ---- 変数 ----
	// {{変数名}} の形で参照される。自分自身以外の全エンティティの JSON を探す。
	// Google タグ設定（gtagConfig）も変数を参照するので含める。
	corpus := marshalAll(v.Tag, v.Trigger, v.Variable, v.Client, v.Zone, v.Transformation, v.GtagConfig)
	for _, vr := range v.Variable {
		needle := jsonEscape("{{" + vr.Name + "}}")
		total := strings.Count(corpus, needle)
		self := strings.Count(marshalAll(vr), needle)
		if total-self == 0 {
			a.add(CodeUnusedVariable, "info", "variable", vr.VariableId, vr.Name, "not referenced as {{"+vr.Name+"}} by any tag, trigger, variable, client, transformation, zone or Google tag config")
		}
	}

	for _, f := range BrokenVariableReferences(v) {
		a.Findings = append(a.Findings, f)
	}

	for _, tc := range types {
		a.TagTypes = append(a.TagTypes, *tc)
	}
	sort.Slice(a.TagTypes, func(i, j int) bool {
		if a.TagTypes[i].Count != a.TagTypes[j].Count {
			return a.TagTypes[i].Count > a.TagTypes[j].Count
		}
		return a.TagTypes[i].Type < a.TagTypes[j].Type
	})
	sort.SliceStable(a.CustomHTML, func(i, j int) bool { return a.CustomHTML[i].Bytes > a.CustomHTML[j].Bytes })
	sort.SliceStable(a.Findings, func(i, j int) bool {
		if a.Findings[i].Severity != a.Findings[j].Severity {
			return a.Findings[i].Severity == "warn"
		}
		return a.Findings[i].Code < a.Findings[j].Code
	})
	return a
}

var varRefRe = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

// BrokenVariableReferences は、存在しない変数を {{名前}} で参照している要素を返す。
// GTM の API はこれを書き込み時には拒まず、バージョン作成（コンパイル）時に理由なしの
// compilerError にするので、ここで突き合わせて原因を示す。
// 使える名前は、ユーザー定義変数と有効な組み込み変数の名前、および _event のような
// "_" で始まる内部変数。
func BrokenVariableReferences(v *tm.ContainerVersion) []Finding {
	known := map[string]bool{}
	for _, x := range v.Variable {
		known[x.Name] = true
	}
	for _, x := range v.BuiltInVariable {
		known[x.Name] = true
	}
	var out []Finding
	check := func(kind, id, name string, entity any) {
		var missing []string
		seen := map[string]bool{}
		for _, m := range varRefRe.FindAllStringSubmatch(jsonUnescape(marshalAll(entity)), -1) {
			ref := strings.TrimSpace(m[1])
			if known[ref] || strings.HasPrefix(ref, "_") || seen[ref] {
				continue
			}
			seen[ref] = true
			missing = append(missing, "{{"+ref+"}}")
		}
		if len(missing) > 0 {
			out = append(out, Finding{Code: CodeBrokenVarRef, Severity: "warn", Kind: kind, ID: id, Name: name,
				Message: "refers to " + strings.Join(missing, ", ") + ", which is neither a variable nor an enabled built-in variable; the container will not compile"})
		}
	}
	for _, x := range v.Tag {
		check("tag", x.TagId, x.Name, x)
	}
	for _, x := range v.Trigger {
		check("trigger", x.TriggerId, x.Name, x)
	}
	for _, x := range v.Variable {
		check("variable", x.VariableId, x.Name, x)
	}
	for _, x := range v.Client {
		check("client", x.ClientId, x.Name, x)
	}
	for _, x := range v.Transformation {
		check("transformation", x.TransformationId, x.Name, x)
	}
	for _, x := range v.GtagConfig {
		check("gtagConfig", x.GtagConfigId, x.Type, x)
	}
	return out
}

// jsonUnescape は JSON の文字列エスケープ（\" や \u003c）を戻して、本文中の {{...}} を拾えるようにする。
func jsonUnescape(s string) string {
	var out strings.Builder
	dec := json.NewDecoder(strings.NewReader(s))
	for {
		var v any
		if err := dec.Decode(&v); err != nil {
			break
		}
		collectStrings(v, &out)
	}
	return out.String()
}

func collectStrings(v any, out *strings.Builder) {
	switch x := v.(type) {
	case string:
		out.WriteString(x)
		out.WriteByte('\n')
	case []any:
		for _, e := range x {
			collectStrings(e, out)
		}
	case map[string]any:
		for _, e := range x {
			collectStrings(e, out)
		}
	}
}

func (a *Audit) add(code, severity, kind, id, name, msg string) {
	a.Findings = append(a.Findings, Finding{Code: code, Severity: severity, Kind: kind, ID: id, Name: name, Message: msg})
}

// IsBuiltInTrigger は組み込みトリガー（All Pages など）のIDかどうかを返す。
func IsBuiltInTrigger(id string) bool {
	var n int64
	_, err := fmt.Sscan(id, &n)
	return err == nil && n > builtInTriggerMin
}

// TemplateType はカスタムテンプレートを使うタグ・変数の type を返す。
// ギャラリーから入れたものは cvt_<ギャラリーID>（例 cvt_MBTSV）、
// コンテナで作ったものは cvt_<コンテナID>_<テンプレートID>。
func TemplateType(containerID, templateID, galleryTemplateID string) string {
	if galleryTemplateID != "" {
		return "cvt_" + galleryTemplateID
	}
	return "cvt_" + containerID + "_" + templateID
}

func tagTypeLabel(typ string, templates map[string]string) string {
	if l, ok := tagTypeLabels[typ]; ok {
		return l
	}
	if strings.HasPrefix(typ, "cvt_") {
		if name, ok := templates[typ]; ok {
			return "Custom template: " + name
		}
		return "Custom template"
	}
	return typ
}

func paramValue(ps []*tm.Parameter, key string) string {
	for _, p := range ps {
		if p.Key == key {
			return p.Value
		}
	}
	return ""
}

func walkParams(ps []*tm.Parameter, fn func(*tm.Parameter)) {
	for _, p := range ps {
		fn(p)
		walkParams(p.List, fn)
		walkParams(p.Map, fn)
	}
}

func marshalAll(vs ...any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, v := range vs {
		_ = enc.Encode(v)
	}
	return buf.String()
}

// jsonEscape は文字列を JSON 文字列の中身として現れる形にする（引用符は除く）。
func jsonEscape(s string) string {
	out := marshalAll(s)
	out = strings.TrimSpace(out)
	return out[1 : len(out)-1]
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func tagIDs(ts []*tm.Tag) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.TagId
	}
	return out
}

func without(s []string, v string) []string {
	var out []string
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
