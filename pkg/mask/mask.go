// Package mask implements path-level field masking for YAML/JSON files.
//
// .agentignore hides whole files from the agent; masking redacts only a
// specific dotted-path value inside a YAML or JSON file. The masked value is
// replaced with the placeholder string when materializing the workspace and is
// always restored from the main project on sync (so an agent cannot leak
// secrets even by editing the placeholder).
package mask

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agentroom/agentroom/pkg/workspace"
	"gopkg.in/yaml.v3"
)

// PlaceholderString is the literal string written into masked locations.
const PlaceholderString = "***MASKED***"

// MasksVersion is the on-disk schema version for masks.json.
const MasksVersion = 1

// ErrPathNotFound is returned when a dotted path cannot be located in the
// document. Callers (notably `ar mask add`) check for it to reject invalid
// path arguments up front.
var ErrPathNotFound = errors.New("mask: path not found")

// Mask is one file -> list-of-paths entry.
type Mask struct {
	File  string   `json:"file"`
	Paths []string `json:"paths"`
}

// Config is the in-memory representation of .agent/masks.json.
type Config struct {
	Version int    `json:"version"`
	Masks   []Mask `json:"masks"`
}

// Load reads .agent/masks.json. Missing file -> empty config (no error).
func Load(mainPath string) (*Config, error) {
	data, err := os.ReadFile(workspace.MasksPath(mainPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Version: MasksVersion}, nil
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse masks.json: %w", err)
	}
	if c.Version == 0 {
		c.Version = MasksVersion
	}
	return &c, nil
}

// Save writes the config atomically.
func Save(mainPath string, c *Config) error {
	if c == nil {
		c = &Config{Version: MasksVersion}
	}
	if c.Version == 0 {
		c.Version = MasksVersion
	}
	if c.Masks == nil {
		c.Masks = []Mask{}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return workspace.AtomicWrite(workspace.MasksPath(mainPath), data, 0o644)
}

// PathsFor returns the masked paths for a relative file path (slash-separated).
// Returns nil if the file has no masks.
func (c *Config) PathsFor(relPath string) []string {
	if c == nil {
		return nil
	}
	for _, m := range c.Masks {
		if m.File == relPath {
			return m.Paths
		}
	}
	return nil
}

// IsMaskable reports whether the file extension is supported (yaml/yml/json).
func IsMaskable(file string) bool {
	return ExtKind(file) != ""
}

// ExtKind classifies a file by extension. Returns "yaml", "json", or "" when
// unsupported.
func ExtKind(file string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	default:
		return ""
	}
}

// Apply replaces each path in `paths` with the placeholder string and returns
// the re-serialized content. Missing paths are silently ignored.
func Apply(content []byte, kind string, paths []string) ([]byte, error) {
	switch kind {
	case "yaml":
		return applyYAMLPlaceholder(content, paths)
	case "json":
		return applyJSONPlaceholder(content, paths)
	default:
		return nil, fmt.Errorf("mask.Apply: unsupported kind %q", kind)
	}
}

// MergeFromMain returns wsContent with each masked path's value overwritten
// by the corresponding value from mainContent. Used at sync time: the agent's
// writes to non-masked fields are preserved, while masked fields are restored
// to whatever the main project currently has.
//
// If a path is missing on the main side, the workspace value at that path is
// left untouched (we have nothing to restore from).
func MergeFromMain(wsContent, mainContent []byte, kind string, paths []string) ([]byte, error) {
	switch kind {
	case "yaml":
		return mergeYAML(wsContent, mainContent, paths)
	case "json":
		return mergeJSON(wsContent, mainContent, paths)
	default:
		return nil, fmt.Errorf("mask.MergeFromMain: unsupported kind %q", kind)
	}
}

// HasPath checks whether the path exists in the given document.
func HasPath(content []byte, kind string, path string) (bool, error) {
	segs := splitPath(path)
	switch kind {
	case "yaml":
		root, err := parseYAML(content)
		if err != nil {
			return false, err
		}
		return lookupYAML(root, segs) != nil, nil
	case "json":
		var obj any
		if len(bytes.TrimSpace(content)) == 0 {
			return false, nil
		}
		if err := json.Unmarshal(content, &obj); err != nil {
			return false, err
		}
		_, ok := lookupJSON(obj, segs)
		return ok, nil
	default:
		return false, fmt.Errorf("mask.HasPath: unsupported kind %q", kind)
	}
}

// ---------- path helpers ----------

// splitPath splits a dotted path into segments. "users.0.email" -> ["users","0","email"].
// Empty input yields an empty slice.
func splitPath(p string) []string {
	p = strings.TrimSpace(p)
	if p == "" {
		return nil
	}
	return strings.Split(p, ".")
}

// ---------- YAML helpers ----------

func parseYAML(content []byte) (*yaml.Node, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, err
	}
	return &root, nil
}

// docNode returns the inner document content node (skipping DocumentNode wrapper).
func docNode(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil
		}
		return root.Content[0]
	}
	return root
}

// lookupYAML traverses node by segs and returns the terminal value node, or
// nil when any segment is missing.
func lookupYAML(root *yaml.Node, segs []string) *yaml.Node {
	cur := docNode(root)
	if cur == nil || len(segs) == 0 {
		return cur
	}
	for _, seg := range segs {
		switch cur.Kind {
		case yaml.MappingNode:
			found := false
			for i := 0; i+1 < len(cur.Content); i += 2 {
				key := cur.Content[i]
				if key.Value == seg {
					cur = cur.Content[i+1]
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		case yaml.SequenceNode:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(cur.Content) {
				return nil
			}
			cur = cur.Content[idx]
		default:
			return nil
		}
	}
	return cur
}

// scalarStringNode builds a plain string scalar node.
func scalarStringNode(v string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: v,
		Style: yaml.DoubleQuotedStyle,
	}
}

// setYAML replaces the value at segs with replacement. Returns true if the
// path existed.
func setYAML(root *yaml.Node, segs []string, replacement *yaml.Node) bool {
	cur := docNode(root)
	if cur == nil || len(segs) == 0 {
		return false
	}
	for i, seg := range segs {
		last := i == len(segs)-1
		switch cur.Kind {
		case yaml.MappingNode:
			j := -1
			for k := 0; k+1 < len(cur.Content); k += 2 {
				if cur.Content[k].Value == seg {
					j = k + 1
					break
				}
			}
			if j < 0 {
				return false
			}
			if last {
				// Preserve key node; overwrite value node fields in place to
				// keep surrounding comments where possible.
				vn := cur.Content[j]
				vn.Kind = replacement.Kind
				vn.Tag = replacement.Tag
				vn.Value = replacement.Value
				vn.Style = replacement.Style
				vn.Content = replacement.Content
				vn.Anchor = ""
				vn.Alias = nil
				return true
			}
			cur = cur.Content[j]
		case yaml.SequenceNode:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(cur.Content) {
				return false
			}
			if last {
				cur.Content[idx] = replacement
				return true
			}
			cur = cur.Content[idx]
		default:
			return false
		}
	}
	return false
}

func marshalYAML(root *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func applyYAMLPlaceholder(content []byte, paths []string) ([]byte, error) {
	root, err := parseYAML(content)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return content, nil
	}
	changed := false
	for _, p := range paths {
		if setYAML(root, splitPath(p), scalarStringNode(PlaceholderString)) {
			changed = true
		}
	}
	if !changed {
		return content, nil
	}
	return marshalYAML(root)
}

func mergeYAML(wsContent, mainContent []byte, paths []string) ([]byte, error) {
	wsRoot, err := parseYAML(wsContent)
	if err != nil {
		return nil, fmt.Errorf("parse workspace yaml: %w", err)
	}
	mainRoot, err := parseYAML(mainContent)
	if err != nil {
		return nil, fmt.Errorf("parse main yaml: %w", err)
	}
	if wsRoot == nil {
		return wsContent, nil
	}
	changed := false
	for _, p := range paths {
		segs := splitPath(p)
		mainVal := lookupYAML(mainRoot, segs)
		if mainVal == nil {
			continue
		}
		replacement := cloneYAMLNode(mainVal)
		if setYAML(wsRoot, segs, replacement) {
			changed = true
		}
	}
	if !changed {
		return wsContent, nil
	}
	return marshalYAML(wsRoot)
}

func cloneYAMLNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	c := &yaml.Node{
		Kind:  n.Kind,
		Style: n.Style,
		Tag:   n.Tag,
		Value: n.Value,
	}
	if len(n.Content) > 0 {
		c.Content = make([]*yaml.Node, len(n.Content))
		for i, ch := range n.Content {
			c.Content[i] = cloneYAMLNode(ch)
		}
	}
	return c
}

// ---------- JSON helpers ----------

func applyJSONPlaceholder(content []byte, paths []string) ([]byte, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return content, nil
	}
	var obj any
	if err := json.Unmarshal(content, &obj); err != nil {
		return nil, err
	}
	changed := false
	for _, p := range paths {
		if setJSON(&obj, splitPath(p), PlaceholderString) {
			changed = true
		}
	}
	if !changed {
		return content, nil
	}
	return json.MarshalIndent(obj, "", "  ")
}

func mergeJSON(wsContent, mainContent []byte, paths []string) ([]byte, error) {
	if len(bytes.TrimSpace(wsContent)) == 0 {
		return wsContent, nil
	}
	var wsObj any
	if err := json.Unmarshal(wsContent, &wsObj); err != nil {
		return nil, fmt.Errorf("parse workspace json: %w", err)
	}
	var mainObj any
	if len(bytes.TrimSpace(mainContent)) > 0 {
		if err := json.Unmarshal(mainContent, &mainObj); err != nil {
			return nil, fmt.Errorf("parse main json: %w", err)
		}
	}
	changed := false
	for _, p := range paths {
		segs := splitPath(p)
		v, ok := lookupJSON(mainObj, segs)
		if !ok {
			continue
		}
		if setJSON(&wsObj, segs, v) {
			changed = true
		}
	}
	if !changed {
		return wsContent, nil
	}
	return json.MarshalIndent(wsObj, "", "  ")
}

func lookupJSON(obj any, segs []string) (any, bool) {
	if len(segs) == 0 {
		return obj, true
	}
	cur := obj
	for _, seg := range segs {
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[seg]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			cur = v[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

func setJSON(root *any, segs []string, value any) bool {
	if len(segs) == 0 || root == nil {
		return false
	}
	cur := *root
	for i, seg := range segs {
		last := i == len(segs)-1
		switch v := cur.(type) {
		case map[string]any:
			if _, ok := v[seg]; !ok {
				return false
			}
			if last {
				v[seg] = value
				return true
			}
			cur = v[seg]
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(v) {
				return false
			}
			if last {
				v[idx] = value
				return true
			}
			cur = v[idx]
		default:
			return false
		}
	}
	return false
}
