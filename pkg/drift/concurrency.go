package drift

import (
	"context"

	"github.com/moby/patternmatcher"
	"github.com/rs/zerolog/log"
)

// nonConcurrentGroups returns, for each project dir, the indices of the non-concurrent patterns it
// matches. Each pattern is its own group: projects sharing a group must not run at the same time.
// Dirs must be repo-relative — a pattern like "gcp/confluent/**" is a prefix match and would never
// match the absolute paths a --repo-url clone produces.
func nonConcurrentGroups(patterns []string, relDirs []string) ([][]int, error) {
	matchers := make([]*patternmatcher.PatternMatcher, 0, len(patterns))
	for _, pattern := range patterns {
		pm, err := patternmatcher.New([]string{pattern})
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, pm)
	}

	groups := make([][]int, len(relDirs))
	for i, dir := range relDirs {
		for g, pm := range matchers {
			match, err := pm.MatchesOrParentMatches(dir)
			if err != nil {
				return nil, err
			}
			if match {
				groups[i] = append(groups[i], g)
			}
		}
	}
	return groups, nil
}

// waitForRunnable returns the position in pending of the first project none of whose groups is busy,
// blocking on released while every pending project waits on a running one. That wait always ends: a
// busy group means an in-flight worker that will send its groups on released. Returns -1 if ctx is
// cancelled while waiting.
func waitForRunnable(ctx context.Context, pending []int, groups [][]int, busy []bool, released <-chan []int) int {
	drainReleased(released, busy)
	pos := nextRunnable(pending, groups, busy)
	for pos < 0 {
		log.Debug().Msgf("All %d pending project(s) are waiting on non-concurrent projects to finish", len(pending))
		select {
		case <-ctx.Done():
			return -1
		case g := <-released:
			setBusy(g, busy, false)
		}
		drainReleased(released, busy)
		pos = nextRunnable(pending, groups, busy)
	}
	return pos
}

// nextRunnable returns the position in pending of the first project none of whose groups is busy,
// or -1 if there is none. With no groups configured this is always 0, preserving discovery order.
func nextRunnable(pending []int, groups [][]int, busy []bool) int {
	for pos, idx := range pending {
		if !anyBusy(groups[idx], busy) {
			return pos
		}
	}
	return -1
}

// drainReleased frees the groups of every worker that finished since the last call, without blocking.
func drainReleased(released <-chan []int, busy []bool) {
	for {
		select {
		case groups := <-released:
			setBusy(groups, busy, false)
		default:
			return
		}
	}
}

// anyBusy reports whether any of the given groups currently has a project running.
func anyBusy(groups []int, busy []bool) bool {
	for _, g := range groups {
		if busy[g] {
			return true
		}
	}
	return false
}

func setBusy(groups []int, busy []bool, value bool) {
	for _, g := range groups {
		busy[g] = value
	}
}
