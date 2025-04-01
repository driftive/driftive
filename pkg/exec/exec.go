package exec

import (
	"driftive/pkg/models"
	"errors"
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
)

type Executor interface {
	Dir() string
	Init(args ...string) (string, error)
	Plan(args ...string) (string, error)
	ParsePlan(output string) string
	ParseErrorOutput(output string) string
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
	// TERRAGRUNT_FORWARD_TF_STDOUT format is deprecated since v0.73.0
	// Reference: https://github.com/gruntwork-io/terragrunt/releases/tag/v0.73.0
	// FIXME replace this by TG_TF_FORWARD_STDOUT=true when support is dropped
	cmd.Env = append(cmd.Env, "TERRAGRUNT_FORWARD_TF_STDOUT=true")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			log.Debug().Msgf("Error running command in %s: %s %v.\nExit error: %s", dir, name, arg, exiterr)
		} else {
			log.Debug().Msgf("Error running command in %s: %s %v.\nError: %s", dir, name, arg, err)
		}
	}
	return string(out), err
}
