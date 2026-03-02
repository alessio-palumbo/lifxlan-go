package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_classifyTokens(t *testing.T) {
	testCases := map[string]struct {
		words     []string
		selectors map[int]*selectorMatch
		want      []token
	}{
		"single selector": {
			words: []string{"set", "d00000000000", "to", "red"},
			selectors: map[int]*selectorMatch{
				1: {Match: "d00000000000", Span: 1},
			},
			want: []token{
				{Raw: "d00000000000", Kind: tokenSelector},
				{Raw: "red", Kind: tokenColor},
			},
		},
		"single selector, multiple actions": {
			words: []string{"set", "d00000000000", "to", "red", "and", "brightness", "30%"},
			selectors: map[int]*selectorMatch{
				1: {Match: "d00000000000", Span: 1},
			},
			want: []token{
				{Raw: "d00000000000", Kind: tokenSelector},
				{Raw: "red", Kind: tokenColor},
				{Raw: "brightness", Kind: tokenProperty},
				{Raw: "30%", Kind: tokenNumber, Value: 30},
			},
		},
		"multiple selectors": {
			words: []string{"set", "moon", "and", "living", "room", "to", "green"},
			selectors: map[int]*selectorMatch{
				1: {Match: "moon", Span: 1},
				3: {Match: "living room", Span: 2},
			},
			want: []token{
				{Raw: "moon", Kind: tokenSelector},
				{Raw: "living room", Kind: tokenSelector},
				{Raw: "green", Kind: tokenColor},
			},
		},
		"just keywords": {
			words: []string{"home", "green"},
			selectors: map[int]*selectorMatch{
				0: {Match: "home", Span: 0},
			},
			want: []token{
				{Raw: "home", Kind: tokenSelector},
				{Raw: "green", Kind: tokenColor},
			},
		},
		"just keywords: flipped": {
			words: []string{"off", "home"},
			selectors: map[int]*selectorMatch{
				1: {Match: "home", Span: 1},
			},
			want: []token{
				{Raw: "off", Kind: tokenPower},
				{Raw: "home", Kind: tokenSelector},
			},
		},
		"with duration suffix": {
			words: []string{"turn", "luna", "red", "in", "10s"},
			selectors: map[int]*selectorMatch{
				1: {Match: "luna", Span: 1},
			},
			want: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "red", Kind: tokenColor},
				{Raw: "10s", Kind: tokenNumberD, Value: 10, Suffix: "s"},
			},
		},
		"with duration word": {
			words: []string{"turn", "luna", "red", "in", "10", "seconds"},
			selectors: map[int]*selectorMatch{
				1: {Match: "luna", Span: 1},
			},
			want: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "red", Kind: tokenColor},
				{Raw: "10", Kind: tokenNumber, Value: 10},
				{Raw: "seconds", Kind: tokenDuration},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cmdParser := &CommandParser{}
			got := cmdParser.classifyTokens(tc.words, tc.selectors)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_isNumber(t *testing.T) {
	testCases := map[string]struct {
		word       string
		wantValue  int
		wantKind   tokenKind
		wantSuffix string
	}{
		"invalid": {
			word: "two",
		},
		"kelvin suffix": {
			word:       "3500k",
			wantValue:  3500,
			wantKind:   tokenNumberK,
			wantSuffix: "k",
		},
		"seconds suffix": {
			word:       "5s",
			wantValue:  5,
			wantKind:   tokenNumberD,
			wantSuffix: "s",
		},
		"ms suffix": {
			word:       "500ms",
			wantValue:  500,
			wantKind:   tokenNumberD,
			wantSuffix: "ms",
		},
		"% suffix": {
			word:      "50%",
			wantValue: 50,
			wantKind:  tokenNumber,
		},
		"number": {
			word:      "360",
			wantValue: 360,
			wantKind:  tokenNumber,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotV, gotS, gotK := isNumber(tc.word)
			assert.Equal(t, tc.wantValue, gotV)
			assert.Equal(t, tc.wantKind, gotK)
			assert.Equal(t, tc.wantSuffix, gotS)
		})
	}
}
