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

Drift issues open: {{ len .DriftedProjects }}

{{ range $project := .DriftedProjects -}}
* [{{ $project.Project.Dir }}](../issues/{{ $project.Issue.Number }})
{{ end -}}
{{ else -}}
No drifts found.
{{ end -}}

{{if .ErroredProjects -}}
## Errors

Error issues open: {{ len .ErroredProjects }}

{{ range $project := .ErroredProjects -}}
* [{{ $project.Project.Dir }}](../issues/{{ $project.Issue.Number }})
{{ end -}}
{{ end -}}

<!--
summary-state-start
{{ .State }}
summary-state-end
-->
