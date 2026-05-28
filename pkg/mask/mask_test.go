package mask

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExtKind(t *testing.T) {
	cases := map[string]string{
		"a.yaml":         "yaml",
		"a.yml":          "yaml",
		"config/app.YML": "yaml",
		"b.json":         "json",
		"b.JSON":         "json",
		"c.txt":          "",
		"d.toml":         "",
		"noext":          "",
	}
	for file, want := range cases {
		if got := ExtKind(file); got != want {
			t.Errorf("ExtKind(%q) = %q, want %q", file, got, want)
		}
	}
}

func TestIsMaskable(t *testing.T) {
	if !IsMaskable("x.yaml") || !IsMaskable("x.json") {
		t.Fatal("yaml/json must be maskable")
	}
	if IsMaskable("x.txt") || IsMaskable("x.toml") {
		t.Fatal("non-yaml/json must be rejected")
	}
}

func TestApply_YAML(t *testing.T) {
	src := []byte(`name: foo
spring:
  datasource:
    username: admin
    password: secret123
list:
  - id: 1
    token: abc
  - id: 2
    token: def
`)
	out, err := Apply(src, "yaml", []string{
		"spring.datasource.password",
		"list.0.token",
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, PlaceholderString) {
		t.Fatalf("placeholder missing: %s", s)
	}
	if strings.Contains(s, "secret123") {
		t.Fatalf("masked value leaked: %s", s)
	}
	if strings.Contains(s, "abc") {
		t.Fatalf("list.0.token not masked: %s", s)
	}
	if !strings.Contains(s, "admin") {
		t.Fatalf("non-masked username should remain: %s", s)
	}
	if !strings.Contains(s, "def") {
		t.Fatalf("list.1.token should remain: %s", s)
	}

	// Re-applying should be idempotent (hash-stable when content already masked).
	out2, err := Apply(out, "yaml", []string{
		"spring.datasource.password",
		"list.0.token",
	})
	if err != nil {
		t.Fatalf("Apply re-run: %v", err)
	}
	if string(out2) != string(out) {
		t.Fatalf("Apply not idempotent\n--- first ---\n%s\n--- second ---\n%s", out, out2)
	}
}

func TestApply_JSON(t *testing.T) {
	src := []byte(`{"name":"foo","api":{"key":"secret"},"users":[{"email":"a@x"},{"email":"b@x"}]}`)
	out, err := Apply(src, "json", []string{"api.key", "users.0.email"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["api"].(map[string]any)["key"] != PlaceholderString {
		t.Fatalf("api.key not masked: %v", obj)
	}
	users := obj["users"].([]any)
	if users[0].(map[string]any)["email"] != PlaceholderString {
		t.Fatalf("users.0.email not masked: %v", obj)
	}
	if users[1].(map[string]any)["email"] != "b@x" {
		t.Fatalf("users.1.email should remain: %v", obj)
	}
}

func TestApply_MissingPathSilent(t *testing.T) {
	src := []byte("a: 1\nb: 2\n")
	out, err := Apply(src, "yaml", []string{"nonexistent.path"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if string(out) != string(src) {
		t.Fatalf("missing path should leave content unchanged, got %s", out)
	}
}

func TestMergeFromMain_YAML(t *testing.T) {
	main := []byte(`name: foo
password: real_pw
other: m1
`)
	ws := []byte(`name: foo
password: "` + PlaceholderString + `"
other: changed
`)
	out, err := MergeFromMain(ws, main, "yaml", []string{"password"})
	if err != nil {
		t.Fatalf("MergeFromMain: %v", err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["password"] != "real_pw" {
		t.Fatalf("password not restored from main: %v", got)
	}
	if got["other"] != "changed" {
		t.Fatalf("non-masked field should retain ws value: %v", got)
	}
}

func TestMergeFromMain_AgentTamperingDiscarded(t *testing.T) {
	// Even if the agent overwrites the placeholder with a different value,
	// MergeFromMain restores the main value.
	main := []byte(`secret: real_999
name: foo
`)
	ws := []byte(`secret: hacked_by_agent
name: bar
`)
	out, err := MergeFromMain(ws, main, "yaml", []string{"secret"})
	if err != nil {
		t.Fatalf("MergeFromMain: %v", err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["secret"] != "real_999" {
		t.Fatalf("masked path must be restored to main value, got %v", got["secret"])
	}
	if got["name"] != "bar" {
		t.Fatalf("non-masked path must retain ws value, got %v", got["name"])
	}
}

func TestMergeFromMain_JSON(t *testing.T) {
	main := []byte(`{"api":{"key":"REAL"},"name":"foo"}`)
	ws := []byte(`{"api":{"key":"` + PlaceholderString + `"},"name":"bar"}`)
	out, err := MergeFromMain(ws, main, "json", []string{"api.key"})
	if err != nil {
		t.Fatalf("MergeFromMain: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["api"].(map[string]any)["key"] != "REAL" {
		t.Fatalf("api.key not restored: %v", got)
	}
	if got["name"] != "bar" {
		t.Fatalf("name should retain ws value: %v", got)
	}
}

func TestHasPath(t *testing.T) {
	yamlSrc := []byte("a:\n  b: 1\nlist:\n  - id: 1\n")
	jsonSrc := []byte(`{"a":{"b":1},"list":[{"id":1}]}`)

	type tc struct {
		kind, src, path string
		want            bool
	}
	cases := []tc{
		{"yaml", string(yamlSrc), "a.b", true},
		{"yaml", string(yamlSrc), "a.c", false},
		{"yaml", string(yamlSrc), "list.0.id", true},
		{"yaml", string(yamlSrc), "list.9.id", false},
		{"json", string(jsonSrc), "a.b", true},
		{"json", string(jsonSrc), "a.c", false},
		{"json", string(jsonSrc), "list.0.id", true},
	}
	for _, c := range cases {
		got, err := HasPath([]byte(c.src), c.kind, c.path)
		if err != nil {
			t.Fatalf("HasPath(%s,%s): %v", c.kind, c.path, err)
		}
		if got != c.want {
			t.Errorf("HasPath(%s,%s) = %v, want %v", c.kind, c.path, got, c.want)
		}
	}
}

func TestConfig_PathsFor(t *testing.T) {
	var c *Config
	if c.PathsFor("any") != nil {
		t.Fatal("nil config must return nil")
	}
	c = &Config{Masks: []Mask{
		{File: "a.yaml", Paths: []string{"p1", "p2"}},
		{File: "b.json", Paths: []string{"k"}},
	}}
	if got := c.PathsFor("a.yaml"); len(got) != 2 {
		t.Fatalf("a.yaml paths: %v", got)
	}
	if c.PathsFor("missing") != nil {
		t.Fatal("missing file must return nil")
	}
}
