{{- /*
  Plain-markdown twin of a section/list page, served at <section>/index.md.
  Emits the section's own content, then a linked list of its child pages so an
  LLM can navigate the section.

  Auto-generated section nodes have no backing file; RenderShortcodes panics on
  those, so the body is only rendered when the page is file-backed.
*/ -}}
# {{ .Title }}
{{ with .Description }}
> {{ . }}
{{ end }}
{{- with .File }}{{ $.RenderShortcodes }}{{ end }}
{{- with .Pages.ByWeight }}

## Pages in this section
{{ range . }}
- [{{ .Title }}]({{ .Permalink | replaceRE `/$` "/index.md" }}){{ with .Description }}: {{ . | plainify | chomp | safeHTML }}{{ end }}
{{- end }}
{{ end -}}
