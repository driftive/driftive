package exec

type TerragruntExecutor struct {
	dir string
}

func (t TerragruntExecutor) Dir() string {
	return t.dir
}

func (t TerragruntExecutor) Init(args ...string) (string, error) {
	return RunCommandInDir(t.Dir(), "terragrunt", append([]string{"init"}, args...)...)
}

func (t TerragruntExecutor) Plan(args ...string) (string, error) {
	return RunCommandInDir(t.Dir(), "terragrunt", append([]string{"plan"}, args...)...)
}

func (t TerragruntExecutor) ParsePlan(output string) string {
	return parsePlan(output)
}

func (t TerragruntExecutor) ParseErrorOutput(output string) string {
	return parseErrorOutput(output)
}
