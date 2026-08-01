# Driftive Summary

**{{ .TotalProjects }} project{{ if ne .TotalProjects 1 }}s{{ end }}** · 🔴 {{ .NumDrifted }} drifted · 🟠 {{ .NumErrored }} errored · ⏭️ {{ .NumSkipped }} skipped · 🟢 {{ .NumClean }} clean{{ if .NumNotChecked }} · ⚪ {{ .NumNotChecked }} not checked{{ end }}

_Last analysis: {{ .LastAnalysisDisplay }} · took {{ .Duration }}_{{ if .DashboardURL }} · [Dashboard]({{ .DashboardURL }}){{ end }}
{{ if .Drifted }}
## 🔴 Drifted ({{ len .Drifted }})

| Project | Issue |
| --- | --- |
{{ range .Drifted }}| {{ .DirCell }} | {{ .IssueLink }} |
{{ end }}{{ end }}
{{- if .Errored }}
## 🟠 Errored ({{ len .Errored }})

| Project | Failed at | Issue |
| --- | --- | --- |
{{ range .Errored }}| {{ .DirCell }} | {{ .FailedPhase }} | {{ .IssueLink }} |
{{ end }}{{ end }}
{{- if .Skipped }}
## ⏭️ Skipped — open PR ({{ len .Skipped }})

| Project |
| --- |
{{ range .Skipped }}| {{ .DirCell }} |
{{ end }}{{ end }}
{{- if .HasRateLimited }}
> ℹ️ Some issues were not created because the configured `max_open_issues` limit was reached.
{{ end }}
{{- if .OtherIssues }}
## 🗂️ Other open issues ({{ len .OtherIssues }})

Open driftive issues the last run did not reproduce.

| Project | Issue |
| --- | --- |
{{ range .OtherIssues }}| {{ .DirCell }} | {{ .IssueLink }} |
{{ end }}{{ end }}
{{- if not .HasFindings }}
✅ No drift or errors detected.
{{ end }}
<!--
summary-state-start
{{ .State }}
summary-state-end
-->
