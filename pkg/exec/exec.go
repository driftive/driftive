package exec

import (
	"driftive/pkg/models"
	"errors"
	"github.com/rs/zerolog/log"
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
