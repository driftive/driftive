This issue shows your drift summary.

{{if .RateLimitedProjects -}}
## Rate-Limited

Issues for the following projects were not created due to configured rate limits:

{{ range $project := .RateLimitedProjects -}}
* `{{ $project }}`
{{ end -}}
{{ end -}}

{{if .DriftedProjects -}}
## Drifts

{{ len .DriftedProjects }} drift issues are open:

{{ range $project := .DriftedProjects -}}
* `{{ $project }}`
{{ end -}}
{{ else -}}
No drifts found.
{{ end -}}

{{if .ErroredProjects -}}
## Errors

{{ len .ErroredProjects }} error issues are open:

{{ range $project := .ErroredProjects -}}
* `{{ $project }}`
{{ end -}}
{{ end -}}

<!--
summary-state-start
{{ .State }}
summary-state-end
-->
