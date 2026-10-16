package drift

import (
	"reflect"
	"testing"
)

func TestNonConcurrentGroups(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		dirs     []string
		want     [][]int
	}{
		{
			name:     "no patterns",
			patterns: nil,
			dirs:     []string{"gcp/confluent/prod", "."},
			want:     [][]int{nil, nil},
		},
		{
			name:     "double star matches subdirectories only",
			patterns: []string{"gcp/confluent/**"},
			dirs:     []string{"gcp/confluent/prod", "gcp/confluent/prod/kafka", "gcp/confluent", "gcp/other", "."},
			want:     [][]int{{0}, {0}, nil, nil, nil},
		},
		{
			name:     "plain dir matches itself and subdirectories",
			patterns: []string{"gcp/confluent"},
			dirs:     []string{"gcp/confluent", "gcp/confluent/prod", "gcp/confluentx"},
			want:     [][]int{{0}, {0}, nil},
		},
		{
			name:     "overlapping patterns",
			patterns: []string{"gcp/**", "gcp/confluent/**"},
			dirs:     []string{"gcp/confluent/prod", "gcp/other", "aws/x"},
			want:     [][]int{{0, 1}, {0}, nil},
		},
		{
			name:     "wildcard segment",
			patterns: []string{"**/confluent/**"},
			dirs:     []string{"gcp/confluent/prod", "aws/confluent/dev", "aws/kafka"},
			want:     [][]int{{0}, {0}, nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nonConcurrentGroups(tt.patterns, tt.dirs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nonConcurrentGroups() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNonConcurrentGroupsInvalidPattern(t *testing.T) {
	if _, err := nonConcurrentGroups([]string{"["}, []string{"a"}); err == nil {
		t.Error("expected an error for a malformed pattern")
	}
}

func TestNextRunnable(t *testing.T) {
	groups := [][]int{{0}, {0}, nil, {0, 1}}
	busy := []bool{true, false}

	if got := nextRunnable([]int{0, 1, 2, 3}, groups, busy); got != 2 {
		t.Errorf("expected the first project outside the busy group (pos 2), got %d", got)
	}
	if got := nextRunnable([]int{0, 1, 3}, groups, busy); got != -1 {
		t.Errorf("expected -1 when every pending project shares the busy group, got %d", got)
	}
	if got := nextRunnable([]int{3, 2}, groups, []bool{false, false}); got != 0 {
		t.Errorf("expected pending order to be preserved when nothing is busy, got %d", got)
	}
}
