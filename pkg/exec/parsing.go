package exec

import (
	"regexp"
	"strings"
)

const (
	TFTofuChangesRegex    = `(Terraform|OpenTofu) will perform the following actions:`
	TFTofuPlanFailedRegex = `Planning failed. (Terraform|OpenTofu) encountered an error while generating this plan.`
	TfTofuMissingOutArg   = `Note: You didn't use the -out option to save this plan, so.*can't\sguarantee to take exactly these actions if you run ".*apply" now.`
)

func parsePlan(output string) string {
	r, _ := regexp.Compile(TFTofuPlanFailedRegex)
	if r.MatchString(output) {
		planFailedIdx := r.FindStringIndex(output)
		return strings.Trim(output[planFailedIdx[0]:], " \n")
	}

	r, _ = regexp.Compile(TFTofuChangesRegex)
	if r.MatchString(output) {
		planWithChangesIdx := r.FindStringIndex(output)
		partial := strings.Trim(output[planWithChangesIdx[0]:], " \n")
		r, _ = regexp.Compile(TfTofuMissingOutArg)
		if r.MatchString(partial) {
			missingOutArgIdx := r.FindStringIndex(partial)
			return strings.Trim(partial[:missingOutArgIdx[0]], " \n")
		}
		return partial
	}
	return output
}
