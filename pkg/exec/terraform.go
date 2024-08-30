package exec

type TerraformExecutor struct {
	dir string
}

func (t TerraformExecutor) Dir() string {
	return t.dir
}

func (t TerraformExecutor) Init(args ...string) (string, error) {
	return RunCommandInDir(t.Dir(), "terraform", append([]string{"init"}, args...)...)
}

func (t TerraformExecutor) Plan(args ...string) (string, error) {
	return RunCommandInDir(t.Dir(), "terraform", append([]string{"plan"}, args...)...)
}

func (t TerraformExecutor) ParsePlan(output string) string {
	return parsePlan(output)
}

func (t TerraformExecutor) ParseErrorOutput(output string) string {
	return parseErrorOutput(output)
}
