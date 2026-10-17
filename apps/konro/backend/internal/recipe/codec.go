package recipe

import (
	"bytes"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Document keeps unowned metadata and body sections when editable fields change.
// YAML layout may be reformatted; unknown values, comments and aliases survive.
type Document struct {
	meta   *yaml.Node
	body   string
	Recipe Recipe
}

func DecodeDocument(data []byte) (*Document, error) {
	src := strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\ufeff")
	d := &Document{meta: &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, body: src}
	if strings.HasPrefix(src, "---\n") {
		end := strings.Index(src[4:], "\n---\n")
		if end < 0 {
			return nil, fmt.Errorf("%w: frontmatterが閉じていません", ErrInvalid)
		}
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(src[4:4+end]), &node); err != nil {
			return nil, fmt.Errorf("%w: YAMLを読み込めません", ErrInvalid)
		}
		if len(node.Content) > 0 {
			if node.Content[0].Kind != yaml.MappingNode {
				return nil, ErrInvalid
			}
			d.meta = node.Content[0]
		}
		d.body = src[4+end+5:]
	}
	seen := map[string]bool{}
	for i := 0; i < len(d.meta.Content); i += 2 {
		k, n := d.meta.Content[i], d.meta.Content[i+1]
		if k.Kind != yaml.ScalarNode || seen[k.Value] {
			return nil, ErrInvalid
		}
		seen[k.Value] = true
		var target any
		switch k.Value {
		case "title":
			target = &d.Recipe.Title
		case "servings":
			target = &d.Recipe.Servings
		case "source":
			target = &d.Recipe.Source
		case "tags":
			target = &d.Recipe.Tags
		case "images":
			target = &d.Recipe.Images
		case "prep_time":
			target = &d.Recipe.PrepTime
		case "cook_time":
			target = &d.Recipe.CookTime
		case "rating":
			target = &d.Recipe.Rating
		case "rk_id":
			target = &d.Recipe.RKID
		}
		if target != nil && n.Decode(target) != nil {
			return nil, fmt.Errorf("%w: %sの形式を確認してください", ErrInvalid, k.Value)
		}
	}
	parseBody(d.body, &d.Recipe)
	return d, nil
}
func (d *Document) set(key string, value any, flow bool) error {
	var n yaml.Node
	if err := n.Encode(value); err != nil {
		return err
	}
	if flow {
		n.Style = yaml.FlowStyle
		for _, item := range n.Content {
			item.Style = yaml.DoubleQuotedStyle
		}
	}
	if n.Kind == yaml.ScalarNode && n.Tag == "!!str" && key != "type" && key != "rk_id" && key != "created" && key != "updated" && key != "prep_time" && key != "cook_time" {
		n.Style = yaml.DoubleQuotedStyle
	}
	for i := 0; i < len(d.meta.Content); i += 2 {
		if d.meta.Content[i].Value == key {
			old := d.meta.Content[i+1]
			if err := d.detachChildAliases(old); err != nil {
				return err
			}
			n.HeadComment = old.HeadComment
			n.LineComment = old.LineComment
			n.FootComment = old.FootComment
			n.Anchor = old.Anchor
			*old = n
			return nil
		}
	}
	d.meta.Content = append(d.meta.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &n)
	return nil
}

// Child anchors removed by replacing an editable list can still be referenced
// by unowned metadata. Resolve those references before publishing the document.
func (d *Document) detachChildAliases(old *yaml.Node) error {
	children := map[*yaml.Node]bool{}
	var collect func(*yaml.Node)
	collect = func(n *yaml.Node) {
		for _, c := range n.Content {
			children[c] = true
			collect(c)
		}
	}
	collect(old)
	var clone func(*yaml.Node, map[*yaml.Node]bool) (*yaml.Node, error)
	clone = func(n *yaml.Node, path map[*yaml.Node]bool) (*yaml.Node, error) {
		if path[n] {
			return nil, fmt.Errorf("%w: 循環するYAML参照は編集できません", ErrInvalid)
		}
		path[n] = true
		defer delete(path, n)
		if n.Kind == yaml.AliasNode {
			return clone(n.Alias, path)
		}
		copy := *n
		copy.Anchor = ""
		copy.Content = nil
		for _, c := range n.Content {
			v, err := clone(c, path)
			if err != nil {
				return nil, err
			}
			copy.Content = append(copy.Content, v)
		}
		return &copy, nil
	}
	var walk func(*yaml.Node) error
	walk = func(n *yaml.Node) error {
		if n.Kind == yaml.AliasNode && children[n.Alias] {
			v, err := clone(n.Alias, map[*yaml.Node]bool{})
			if err != nil {
				return err
			}
			v.HeadComment = n.HeadComment
			v.LineComment = n.LineComment
			v.FootComment = n.FootComment
			*n = *v
		}
		for _, c := range n.Content {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(d.meta)
}

func (d *Document) Edited() bool {
	for i := 0; i < len(d.meta.Content); i += 2 {
		if d.meta.Content[i].Value == "konro_edited" {
			var b bool
			return d.meta.Content[i+1].Decode(&b) == nil && b
		}
	}
	return false
}
func (d *Document) setEditable(e Editable) error {
	values := []struct {
		key   string
		value any
		flow  bool
	}{
		{"title", e.Title, false}, {"servings", e.Servings, false}, {"source", e.Source, false}, {"tags", e.Tags, true}, {"prep_time", e.PrepTime, false}, {"cook_time", e.CookTime, false}, {"rating", e.Rating, false},
	}
	for _, v := range values {
		if err := d.set(v.key, v.value, v.flow); err != nil {
			return err
		}
	}
	return nil
}
func (d *Document) render(e Editable, date string, edited bool) ([]byte, error) {
	if err := d.setEditable(e); err != nil {
		return nil, err
	}
	if date != "" {
		_ = d.set("updated", date, false)
	}
	if edited {
		_ = d.set("konro_edited", true, false)
	}
	body, err := replaceBody(d.body, e)
	if err != nil {
		return nil, err
	}
	return d.encode(body)
}
func (d *Document) encode(body string) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("---\n")
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(d.meta); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	b.WriteString("---\n")
	b.WriteString(body)
	return b.Bytes(), nil
}

// Encode creates the canonical file format for all recipe entry points.
func Encode(r Recipe, date string) ([]byte, error) {
	d := &Document{meta: &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}}
	_ = d.set("type", "recipe", false)
	_ = d.set("title", r.Title, false)
	if r.RKID != "" {
		_ = d.set("rk_id", r.RKID, false)
	}
	if len(r.Images) > 0 {
		_ = d.set("images", r.Images, true)
	}
	if date != "" {
		_ = d.set("created", date, false)
	}
	e := normalizeEdit(editable(&r))
	if err := d.setEditable(e); err != nil {
		return nil, err
	}
	if date != "" {
		_ = d.set("updated", date, false)
	}
	// Keep unset optional metadata absent in newly created files.
	for _, key := range []string{"servings", "source", "prep_time", "cook_time", "rating"} {
		for i := 0; i < len(d.meta.Content); i += 2 {
			if d.meta.Content[i].Value == key && (d.meta.Content[i+1].Value == "" || key == "rating" && r.Rating == 0) {
				d.meta.Content = append(d.meta.Content[:i], d.meta.Content[i+2:]...)
				break
			}
		}
	}
	body, err := replaceBody("", e)
	if err != nil {
		return nil, err
	}
	b, err := d.encode(body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// The body contract is one ingredient per bullet and one operation per numbered
// line. Notes consume the remainder (including Markdown headings).
func replaceBody(body string, e Editable) (string, error) {
	contents := map[string]string{"材料": "", "手順": "", "メモ": e.Notes + "\n"}
	var b strings.Builder
	for _, s := range e.Ingredients {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	contents["材料"] = b.String()
	b.Reset()
	for i, s := range e.Steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	contents["手順"] = b.String()
	lines := strings.Split(body, "\n")
	seen := map[string]bool{}
	var out strings.Builder
	for i := 0; i < len(lines); {
		title := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), "## "))
		_, known := contents[title]
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") || !known {
			out.WriteString(lines[i] + "\n")
			i++
			continue
		}
		if seen[title] {
			return "", fmt.Errorf("%w: 同じ本文見出しが複数あります", ErrInvalid)
		}
		seen[title] = true
		j := i + 1
		if title == "メモ" {
			j = len(lines)
		} else {
			for j < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[j]), "## ") {
				t := strings.TrimSpace(lines[j])
				if t != "" && ((title == "材料" && !strings.HasPrefix(t, "- ")) || (title == "手順" && !stepLine.MatchString(t))) {
					return "", fmt.Errorf("%w: 材料と手順は1項目1行で記録してください", ErrInvalid)
				}
				j++
			}
		}
		// A memo owns the remaining body. Insert missing structured sections before it.
		if title == "メモ" {
			for _, missing := range []string{"材料", "手順"} {
				if !seen[missing] {
					out.WriteString("## " + missing + "\n\n" + contents[missing] + "\n")
					seen[missing] = true
				}
			}
		}
		out.WriteString("## " + title + "\n\n" + contents[title] + "\n")
		i = j
	}
	for _, title := range []string{"材料", "手順", "メモ"} {
		if !seen[title] {
			out.WriteString("\n## " + title + "\n\n" + contents[title] + "\n")
		}
	}
	return "\n" + strings.TrimSpace(out.String()) + "\n", nil
}
