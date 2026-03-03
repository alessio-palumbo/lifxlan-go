package command

import (
	"sort"
	"strings"
)

const (
	exactMatchScore    = 100
	prefixMatchScore   = 75
	containsMatchScore = 50
	fuzzyMatchMaxScore = 25

	maxEarlyStartBonus  = 6
	maxTightSpanBonus   = 6
	maxConsecutiveBonus = 5
	maxDensityBonus     = 8

	defaultMatchResultLimit = 10
)

// Match returns a list of selector keys that match the given search term.
// It applies multiple heuristics (exact, prefix, contains, fuzzy) to score candidates
// and returns them sorted by descending score (best matches first).
// If multiple candidates have the same score, they are sorted alphabetically.
// The result is capped by defaultMatchResultLimit if > 0.
func (p *CommandParser) Match(term string) []string {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return nil
	}

	type candidate struct {
		key   string
		score int
	}

	var results []candidate

	for key := range p.selectors {
		score := matchScore(key, term)
		if score > 0 {
			results = append(results, candidate{
				key:   key,
				score: score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return results[i].key < results[j].key
		}
		return results[i].score > results[j].score
	})

	if defaultMatchResultLimit > 0 && len(results) > defaultMatchResultLimit {
		results = results[:defaultMatchResultLimit]
	}

	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.key
	}

	return out
}

// matchScore computes a numeric score for how well `term` matches `key`.
// Higher scores indicate a better match.
// Both arguments are expected to be pre-lowercased.
func matchScore(key, term string) int {
	if key == term {
		return exactMatchScore
	}

	if strings.HasPrefix(key, term) {
		return prefixMatchScore
	}

	if strings.Contains(key, term) {
		return containsMatchScore
	}

	return min(fuzzyScore(key, term), fuzzyMatchMaxScore)
}

// fuzzyScore computes a heuristic score for "fuzzy" matches, where the characters
// of `term` appear in order within `key`, possibly non-consecutively.
// The score is composed of several components:
// 1. Early start bonus: rewards matches that occur early in the key
// 2. Tight span bonus: rewards matches where term letters are close together
// 3. Consecutive match bonus: rewards sequences of consecutive matched characters
// 4. Density bonus: rewards matches that cover a large portion of the matched span
// Both arguments are expected to be pre-lowercased.
func fuzzyScore(key, term string) int {
	if term == "" {
		return 0
	}

	j := 0
	start := -1
	lastMatch := -1
	consecutive := 0

	for i := 0; i < len(key) && j < len(term); i++ {
		if key[i] == term[j] {
			if start == -1 {
				start = i
			}
			if lastMatch == i-1 {
				consecutive++
			}
			lastMatch = i
			j++
		}
	}

	if j != len(term) {
		return 0
	}

	var score int

	if start < maxEarlyStartBonus {
		score += maxEarlyStartBonus - start
	}

	span := lastMatch - start + 1
	extra := span - len(term)
	if extra < maxTightSpanBonus {
		score += maxTightSpanBonus - extra
	}
	score += min(consecutive, maxConsecutiveBonus)

	densityBonus := (len(term) * maxDensityBonus) / span
	score += min(densityBonus, maxDensityBonus)

	return score
}
