package config

type ProjectType int

const (
	Terraform ProjectType = iota
	Tofu
	Terragrunt
)

type Project struct {
	Dir     string      `json:"dir" yaml:"dir"`
	Type    ProjectType `json:"type" yaml:"type"`
	Ignored bool        `json:"ignored" yaml:"ignored"`
}
