package exec

import (
	"driftive/pkg/models"
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
)

type Executor interface {
	Dir() string
	Init(args ...string) (string, error)
	Plan(args ...string) (string, error)
	ParsePlan(output string) string
}

func NewExecutor(dir string, t models.ProjectType) Executor {
	switch t {
	case models.Terraform:
		return TerraformExecutor{dir}
	case models.Terragrunt:
		return TerragruntExecutor{dir}
	case models.Tofu:
		return TofuExecutor{dir}
	default:
		return nil
	}
}

func RunCommand(name string, arg ...string) (string, error) {
	log.Debug().Msgf("Running command: %s %v", name, arg)
	cmd := exec.Command(name, arg...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func RunCommandInDir(dir, name string, arg ...string) (string, error) {
	log.Debug().Msgf("Running command in %s: %s %v", dir, name, arg)
	cmd := exec.Command(name, arg...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TERRAGRUNT_FORWARD_TF_STDOUT=true")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
