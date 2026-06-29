{{- /*
  Plain-markdown twin of a section/list page, served at <section>/index.md.
  Emits the section's own content, then a linked list of its child pages so an
  LLM can navigate the section.
*/ -}}
# {{ .Title }}
{{ with .Description }}
> {{ . }}
{{ end }}
{{/* Auto-generated section nodes have no backing file; RenderShortcodes panics
     on those, so only render a body when the page is file-backed. */}}
{{- with .File }}{{ $.RenderShortcodes }}{{ end }}
{{ with .Pages }}
## Pages in this section
{{ range . }}
- [{{ .Title }}]({{ .Permalink | replaceRE `/$` "/index.md" }})
{{- end }}
{{ end }}
