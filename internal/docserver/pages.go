package docserver

import (
	"html/template"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/annotations"
	"github.com/Lanterna-Games/defold-tools/internal/docs"
)

const referenceOnTheWeb = "https://defold.com/ref/"

type versionPage struct {
	Version  string
	Versions []string
	Modules  []docs.Module
}

type modulePage struct {
	Version  string
	Versions []string
	Module   docs.Module
	OnTheWeb string
}

func markup(text string) template.HTML {
	return template.HTML(text) //nolint:gosec // the text is the documentation Defold publishes, read from its archive or from the copy this program carries
}

func signature(function docs.Function) string {
	names := make([]string, 0, len(function.Parameters))
	for _, parameter := range function.Parameters {
		name := parameter.Name
		if parameter.IsOptional {
			name = "[" + name + "]"
		}
		names = append(names, name)
	}
	return function.Name + "(" + strings.Join(names, ", ") + ")"
}

var pages = template.Must(template.New("pages").Funcs(template.FuncMap{
	"markup":           markup,
	"signature":        signature,
	"types":            func(types []string) string { return strings.Join(types, " | ") },
	"isForGameScripts": annotations.IsWritten,
	"messageAnchor":    docs.AnchorOfTheMessage,
	"propertyAnchor":   docs.AnchorOfTheProperty,
	"dict": func(pairs ...any) map[string]any {
		named := map[string]any{}
		for index := 0; index+1 < len(pairs); index += 2 {
			if name, isAName := pairs[index].(string); isAName {
				named[name] = pairs[index+1]
			}
		}
		return named
	},
}).Parse(`
{{define "head"}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.}}</title>
<style>
:root { color-scheme: light dark; --ink: #1a1a1a; --paper: #fbfaf8; --soft: #6b6b6b; --line: #dddad3; --mark: #c0392b; --code: #f1eee8; }
@media (prefers-color-scheme: dark) { :root { --ink: #e8e6e1; --paper: #151515; --soft: #9a9790; --line: #33312d; --mark: #e2574c; --code: #1f1e1c; } }
* { box-sizing: border-box; }
body { margin: 0; font: 16px/1.55 system-ui, sans-serif; color: var(--ink); background: var(--paper); }
header { padding: 14px 24px; border-bottom: 1px solid var(--line); display: flex; gap: 16px; align-items: baseline; flex-wrap: wrap; }
header a { color: var(--ink); text-decoration: none; font-weight: 600; }
header .where { color: var(--soft); }
header select { margin-left: auto; font: inherit; color: var(--ink); background: var(--paper); border: 1px solid var(--line); border-radius: 4px; padding: 2px 6px; }
main { max-width: 900px; margin: 0 auto; padding: 24px; }
h1 { font-size: 28px; margin: 8px 0 4px; }
h2 { font-size: 20px; margin: 40px 0 8px; padding-bottom: 6px; border-bottom: 1px solid var(--line); }
h3 { font: 600 16px/1.4 ui-monospace, monospace; margin: 32px 0 6px; overflow-wrap: anywhere; }
h3 a { color: var(--mark); text-decoration: none; }
a { color: var(--mark); }
code, pre { font: 14px/1.5 ui-monospace, monospace; background: var(--code); border-radius: 4px; }
code { padding: 1px 4px; }
pre { padding: 12px 14px; overflow-x: auto; }
pre code { padding: 0; background: none; }
table { border-collapse: collapse; width: 100%; margin: 8px 0; }
td, th { text-align: left; vertical-align: top; padding: 6px 12px 6px 0; border-top: 1px solid var(--line); }
td:first-child { white-space: nowrap; font: 14px ui-monospace, monospace; }
.soft { color: var(--soft); }
.modules { columns: 2; padding: 0; list-style: none; }
.modules li { break-inside: avoid; padding: 4px 0; }
.label { font-size: 13px; text-transform: uppercase; letter-spacing: .05em; color: var(--soft); margin: 14px 0 2px; }
</style>
</head>
<body>{{end}}

{{define "versions"}}<select onchange="location.href=this.value">
{{$current := .Version}}{{$suffix := .Suffix}}{{range .Versions}}<option value="/{{.}}/{{$suffix}}"{{if eq . $current}} selected{{end}}>Defold {{.}}</option>
{{end}}</select>{{end}}

{{define "version"}}{{template "head" (printf "Defold %s reference" .Version)}}
<header><a href="/{{.Version}}/">Defold reference</a><span class="where">kept on this machine</span>{{template "versions" (dict "Version" .Version "Versions" .Versions "Suffix" "")}}</header>
<main>
<h1>Defold {{.Version}}</h1>
<p class="soft">The API reference of this version, as Defold publishes it.</p>
<h2>For game scripts</h2>
<ul class="modules">
{{range .Modules}}{{if isForGameScripts .Namespace}}<li><a href="/{{$.Version}}/{{.Namespace}}"><code>{{.Namespace}}</code></a> <span class="soft">{{.Name}}</span></li>
{{end}}{{end}}</ul>
<h2>Lua and the editor</h2>
<ul class="modules">
{{range .Modules}}{{if not (isForGameScripts .Namespace)}}<li><a href="/{{$.Version}}/{{.Namespace}}"><code>{{.Namespace}}</code></a> <span class="soft">{{.Name}}</span></li>
{{end}}{{end}}</ul>
</main>
</body>
</html>{{end}}

{{define "module"}}{{template "head" (printf "%s · Defold %s" .Module.Namespace .Version)}}
<header><a href="/{{.Version}}/">Defold reference</a><span class="where">{{.Module.Namespace}}</span>{{template "versions" (dict "Version" .Version "Versions" .Versions "Suffix" .Module.Namespace)}}</header>
<main>
<h1>{{.Module.Name}}</h1>
<p class="soft"><code>{{.Module.Namespace}}</code> · Defold {{.Version}} · <a href="{{.OnTheWeb}}">on defold.com</a></p>
{{markup .Module.Description}}

{{if .Module.Functions}}<h2>Functions</h2>
{{range .Module.Functions}}<h3 id="{{.Name}}"><a href="#{{.Name}}">{{signature .}}</a></h3>
{{markup .Description}}
{{if .Parameters}}<div class="label">Parameters</div>
<table>{{range .Parameters}}<tr><td>{{.Name}}{{if .IsOptional}} <span class="soft">optional</span>{{end}}</td><td><code>{{types .Types}}</code></td><td>{{markup .Doc}}</td></tr>
{{end}}</table>{{end}}
{{if .Returns}}<div class="label">Returns</div>
<table>{{range .Returns}}<tr><td>{{.Name}}</td><td><code>{{types .Types}}</code></td><td>{{markup .Doc}}</td></tr>
{{end}}</table>{{end}}
{{if .Examples}}<div class="label">Examples</div>
{{markup .Examples}}{{end}}
{{end}}{{end}}

{{if .Module.Messages}}<h2>Messages</h2>
{{range .Module.Messages}}<h3 id="{{messageAnchor .Name}}"><a href="#{{messageAnchor .Name}}">{{.Name}}</a></h3>
{{markup .Description}}
{{if .Fields}}<div class="label">Fields</div>
<table>{{range .Fields}}<tr><td>{{.Name}}</td><td><code>{{types .Types}}</code></td><td>{{markup .Doc}}</td></tr>
{{end}}</table>{{end}}
{{if .Examples}}<div class="label">Examples</div>
{{markup .Examples}}{{end}}
{{end}}{{end}}

{{if .Module.Properties}}<h2>Properties</h2>
{{range .Module.Properties}}<h3 id="{{propertyAnchor .Name}}"><a href="#{{propertyAnchor .Name}}">{{.Name}}</a></h3>
{{markup .Brief}}
{{markup .Description}}
{{if .Examples}}<div class="label">Examples</div>
{{markup .Examples}}{{end}}
{{end}}{{end}}

{{if .Module.Constants}}<h2>Constants</h2>
<table>{{range .Module.Constants}}<tr id="{{.Name}}"><td>{{.Name}}</td><td>{{if .Type}}<code>{{.Type}}</code>{{end}}</td><td>{{markup .Doc}}</td></tr>
{{end}}</table>{{end}}

{{if .Module.Types}}<h2>Types</h2>
{{range .Module.Types}}<h3 id="{{.Name}}"><a href="#{{.Name}}">{{.Name}}</a></h3>
{{markup .Doc}}
{{if .Fields}}<table>{{range .Fields}}<tr><td>{{.Name}}{{if .IsOptional}} <span class="soft">optional</span>{{end}}</td><td><code>{{types .Types}}</code></td><td>{{markup .Doc}}</td></tr>
{{end}}</table>{{end}}
{{end}}{{end}}
</main>
</body>
</html>{{end}}
`))
