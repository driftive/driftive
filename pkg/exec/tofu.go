package exec

type TofuExecutor struct {
	dir string
}

func (t TofuExecutor) Dir() string {
	return t.dir
}

func (t TofuExecutor) Init(args ...string) (string, error) {
	return RunCommandInDir(t.Dir(), "tofu", append([]string{"init"}, args...)...)
}

func (t TofuExecutor) Plan(args ...string) (string, error) {
	return RunCommandInDir(t.Dir(), "tofu", append([]string{"plan"}, args...)...)
}
