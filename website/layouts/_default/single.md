{{- /*
  Plain-markdown twin of a page, served at <page>/index.md for LLM consumption.
  Emits a small header (title, description) followed by the page
  body. RenderShortcodes resolves Hugo shortcodes (e.g. script, ref) in place
  while leaving the surrounding markdown intact.
*/ -}}
# {{ .Title }}
{{ with .Description }}
> {{ . }}
{{ end }}
{{- with .File }}{{ $.RenderShortcodes }}{{ end }}
