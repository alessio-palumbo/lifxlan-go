package command

import (
	"strconv"
	"strings"
)

// tokenKind define the semantic token type.
type tokenKind int

const (
	tokenUnknown tokenKind = iota
	tokenColor
	tokenPower
	tokenNumber
	tokenNumberK
	tokenNumberD
	tokenDuration
	tokenProperty
	tokenSelector
	tokenSeparator
)

// token stores word semantics.
type token struct {
	Raw    string
	Kind   tokenKind
	Value  int
	Suffix string
}

// classifyTokens parses a list words into semantic tokens.
func (p *CommandParser) classifyTokens(words []string, selectorsMatches map[int]*selectorMatch) []token {
	out := make([]token, 0, len(words))

	for i := 0; i < len(words); i++ {
		w := words[i]
		t := token{Raw: w}

		if match := selectorsMatches[i]; match != nil {
			t.Raw = match.Match
			t.Kind = tokenSelector
			out = append(out, t)
			if skipN := match.Span - 1; skipN > 0 {
				i += skipN
			}
			continue
		}

		if _, ok := propertyWords[w]; ok {
			t.Kind = tokenProperty
			out = append(out, t)
			continue
		}

		if _, ok := colorWords[w]; ok {
			t.Kind = tokenColor
			out = append(out, t)
			continue
		}

		if _, ok := durationWords[w]; ok {
			t.Kind = tokenDuration
			out = append(out, t)
			continue
		}

		if w == "on" || w == "off" {
			t.Kind = tokenPower
			out = append(out, t)
			continue
		}

		if v, s, kind := isNumber(w); kind != tokenUnknown {
			t.Kind = kind
			t.Value = v
			t.Suffix = s
			out = append(out, t)
			continue
		}

		if w == nextCommandToken {
			t.Kind = tokenSeparator
			out = append(out, t)
			continue
		}
	}

	return out
}

// isNumber checks whether the word is a number and returns it along with
// any suffix, if supported.
func isNumber(word string) (value int, suffix string, kind tokenKind) {
	if v, s, ok := tokenTypeForSuffix(word, "k"); ok {
		return v, s, tokenNumberK
	}
	// Order matter, match ms before s.
	if v, s, ok := tokenTypeForSuffix(word, "ms", "s", "m"); ok {
		return v, s, tokenNumberD
	}

	// Treat number and percentage the same.
	v, err := strconv.Atoi(strings.TrimSuffix(word, "%"))
	if err == nil {
		return v, "", tokenNumber
	}
	return 0, "", tokenUnknown
}

// tokenTypeForSuffix checks whether a word is number with any of the given
// suffixes and returns them.
func tokenTypeForSuffix(word string, suffixes ...string) (int, string, bool) {
	for _, s := range suffixes {
		if strings.HasSuffix(word, s) {
			v, err := strconv.Atoi(strings.TrimSuffix(word, s))
			if err == nil {
				return v, s, true
			}
		}
	}
	return 0, "", false
}
