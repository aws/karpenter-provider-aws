{{- /*
  Plain-markdown twin of a page, served at <page>/index.md for LLM consumption.
  Emits a small header (title, source URL, description) followed by the page
  body. RenderShortcodes resolves Hugo shortcodes (e.g. script, ref) in place
  while leaving the surrounding markdown intact.
*/ -}}
# {{ .Title }}
{{ with .Description }}
> {{ . }}
{{ end }}
Source: {{ .Permalink | replaceRE `index\.md$` "" }}

{{- with .File }}{{ $.RenderShortcodes }}{{ end }}
