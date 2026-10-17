package webpage

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ScriptObject decodes only a JSON object after a named assignment; never evaluates JS.
func ScriptObject(html, name string) (map[string]any, bool) {
	if len(name) > 80 {
		return nil, false
	}
	marker := regexp.MustCompile(`(?:var\s+)?` + regexp.QuoteMeta(name) + `\s*=\s*`)
	for _, index := range marker.FindAllStringIndex(html, 8) {
		var v map[string]any
		if json.NewDecoder(strings.NewReader(html[index[1]:])).Decode(&v) == nil && v != nil {
			return v, true
		}
	}
	return nil, false
}
