package models

type ProjectType int

const (
	Terraform ProjectType = iota
	Tofu
	Terragrunt
)

// Project represents a TF/Tofu/Terragrunt project to be analyzed
type Project struct {
	Dir  string      `json:"dir" yaml:"dir"`
	Type ProjectType `json:"type" yaml:"type"`
}
