package web

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

type entry struct {
	Name, Size, Modified string
	Link                 template.URL
	Dir                  bool
	Inline               bool
}
type page struct {
	Path      string
	Parent    template.URL
	Entries   []entry
	Truncated bool
}

var listing = template.Must(template.New("listing").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Index of {{.Path}}</title><link rel="stylesheet" href="/_nais/autoindex.css"><link rel="icon" href="/_nais/favicon.ico"></head><body><h1><span>Index of {{.Path}}</span></h1><pre>{{if .Parent}}<a href="../">../</a>
{{end}}{{range .Entries}}<a href="{{.Link}}"{{if not .Dir}} data-file="true"{{if not .Inline}} download{{end}}{{end}}>{{.Name}}{{if .Dir}}/{{end}}</a>  {{.Modified}}  {{.Size}}
{{else}}This folder is empty.
{{end}}{{if .Truncated}}Only the first 5,000 entries are displayed.
{{end}}</pre><script src="/_nais/autoindex.js" defer></script></body></html>`))

func escapedPath(p string) template.URL {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return template.URL(strings.Join(parts, "/"))
}
func sizeLabel(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	for _, u := range []struct {
		n int64
		s string
	}{{1 << 30, "GiB"}, {1 << 20, "MiB"}, {1 << 10, "KiB"}} {
		if n >= u.n {
			return fmt.Sprintf("%.1f %s", float64(n)/float64(u.n), u.s)
		}
	}
	return ""
}
