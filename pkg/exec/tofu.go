package exec

import "context"

type TofuExecutor struct {
	dir string
}

func (t TofuExecutor) Dir() string {
	return t.dir
}

func (t TofuExecutor) Init(args ...string) (string, error) {
	return RunCommandInDir(context.Background(), t.Dir(), "tofu", append([]string{"init"}, args...)...)
}

func (t TofuExecutor) Plan(args ...string) (string, error) {
	return RunCommandInDir(context.Background(), t.Dir(), "tofu", append([]string{"plan"}, args...)...)
}

func (t TofuExecutor) ParsePlan(output string) string {
	return parsePlan(output)
}

func (t TofuExecutor) ParseErrorOutput(output string) string {
	return parseErrorOutput(output)
}
