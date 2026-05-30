package exec

import "context"

type TerraformExecutor struct {
	dir string
}

func (t TerraformExecutor) Dir() string {
	return t.dir
}

func (t TerraformExecutor) Init(ctx context.Context, args ...string) (string, error) {
	return RunCommandInDir(ctx, t.Dir(), "terraform", append([]string{"init"}, args...)...)
}

func (t TerraformExecutor) Plan(ctx context.Context, args ...string) (string, error) {
	return RunCommandInDir(ctx, t.Dir(), "terraform", append([]string{"plan"}, args...)...)
}

func (t TerraformExecutor) ParsePlan(output string) string {
	return parsePlan(output)
}

func (t TerraformExecutor) ParseErrorOutput(output string) string {
	return parseErrorOutput(output)
}
