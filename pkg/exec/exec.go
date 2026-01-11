package exec

import (
	"context"
	"driftive/pkg/models"
	"errors"
	"os"
	"os/exec"

	"github.com/rs/zerolog/log"
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

func RunCommand(ctx context.Context, name string, arg ...string) (string, error) {
	log.Debug().Msgf("Running command: %s %v", name, arg)
	cmd := exec.CommandContext(ctx, name, arg...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func RunCommandInDir(ctx context.Context, dir, name string, arg ...string) (string, error) {
	log.Debug().Msgf("Running command in %s: %s %v", dir, name, arg)
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TG_TF_FORWARD_STDOUT=true")
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
