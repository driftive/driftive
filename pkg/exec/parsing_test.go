package exec

import (
	"driftive/pkg/models"
	"driftive/pkg/utils"
	"strings"
	"testing"
)

func TestErrorOutput(t *testing.T) {
	file := utils.GetTestFile("test/output/error_planning.txt")
	expected := utils.GetTestFile("test/output/error_planning_expected.txt")
	tf := NewExecutor("testdata", models.Terraform)
	result := tf.ParsePlan(string(file))
	if result != strings.Trim(string(expected), " \n") {
		t.Fatalf("Expected: %s\nGot: %s", string(expected), result)
	}
}

func TestChangesOutput(t *testing.T) {
	file := utils.GetTestFile("test/output/changes.txt")
	expected := utils.GetTestFile("test/output/changes_expected.txt")
	tf := NewExecutor("testdata", models.Terraform)
	result := tf.ParsePlan(string(file))
	if result != strings.Trim(string(expected), " \n") {
		t.Fatalf("Expected: %s\nGot: %s", string(expected), result)
	}
}
