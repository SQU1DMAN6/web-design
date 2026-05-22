package template

import (
	"encoding/json"
	"strings"
	"text/template"
)

var Funcs = template.FuncMap{
	"contains":  strings.Contains,
	"hasPrefix": strings.HasPrefix,
	"hasSuffix": strings.HasSuffix,
	"toJSON": func(v interface{}) string {
		data, err := json.Marshal(v)
		if err != nil {
			return "null"
		}
		return string(data)
	},
	"truncate": func(s string, limit int) string {
		if limit <= 0 {
			return ""
		}
		runes := []rune(s)
		if len(runes) <= limit {
			return s
		}
		if limit <= 3 {
			return string(runes[:limit])
		}
		return string(runes[:limit-3]) + "..."
	},
	"trimSuffix": strings.TrimSuffix,
	"trimPrefix": strings.TrimPrefix,
}
