This issue shows your drift summary.

{{if .RateLimitedProjects -}}
## Rate-Limited

These issues are currently rate-limited.
{{ range $project := .RateLimitedProjects -}}
* `{{ $project }}`
{{ end -}}
{{ end -}}

{{if .DriftedProjects -}}
## Drifts

These issues show the drifts between the desired and actual state of your resources.
{{ range $project := .DriftedProjects -}}
* `{{ $project }}`
{{ end -}}
{{ else -}}
No drifts found.
{{ end -}}

{{if .ErroredProjects -}}
## Errors

These issues show the errors that occurred during the drift analysis.
{{ range $project := .ErroredProjects -}}
* `{{ $project }}`
{{ end -}}
{{ end -}}

<!--
summary-state-start
{{ .State }}
summary-state-end
-->
