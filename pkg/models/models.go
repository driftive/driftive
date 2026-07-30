package models

type ProjectType int

const (
	Terraform ProjectType = iota
	Tofu
	Terragrunt
)

type Project struct {
	Dir string `json:"dir" yaml:"dir"`
}

// TypedProject represents a TF/Tofu/Terragrunt project to be analyzed
type TypedProject struct {
	// Dir is the discovered path to the project, carrying whatever prefix --repo-path had
	// (or the temp clone dir under --repo-url). It is used as the subprocess working
	// directory. Results report it relative to the repository root instead.
	Dir  string      `json:"dir" yaml:"dir"`
	Type ProjectType `json:"type" yaml:"type"`
}

func ProjectTypeToStr(t ProjectType) string {
	switch t {
	case Terraform:
		return "tf"
	case Tofu:
		return "tofu"
	case Terragrunt:
		return "tg"
	default:
		return "?"
	}
}
