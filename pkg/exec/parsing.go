package exec

import (
	"github.com/rs/zerolog/log"
	"regexp"
	"strings"
)

const (
	TFTofuChangesRegex    = `(Terraform|OpenTofu) will perform the following actions:`
	TFTofuPlanFailedRegex = `Planning failed. (Terraform|OpenTofu) encountered an error while generating this plan.`
	TfTofuMissingOutArg   = `Note: You didn't use the -out option to save this plan, so.*can't\sguarantee to take exactly these actions if you run ".*apply" now.`
	refreshKeyword        = "Refreshing state..."
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

func parseErrorOutput(output string) string {
	r, _ := regexp.Compile(TFTofuPlanFailedRegex)
	if r.MatchString(output) {
		planFailedIdx := r.FindStringIndex(output)
		log.Debug().Msgf("Plan failed keyword found in error output. Returning output from line %d.", planFailedIdx[0])
		return strings.Trim(output[planFailedIdx[0]:], " \n")
	}

	lines := strings.Split(output, "\n")
	finalIndex := 0
	for i, line := range lines {
		if strings.Contains(line, refreshKeyword) {
			finalIndex = i
		}
	}

	if finalIndex != 0 {
		output = strings.Join(lines[finalIndex+1:], "\n")
		log.Debug().Msgf("Refresh keyword found in error output. Returning output from line %d.", finalIndex+1)
		return output
	}

	log.Debug().Msgf("No refresh keyword found in error output. Returning full output.")
	return output
}
