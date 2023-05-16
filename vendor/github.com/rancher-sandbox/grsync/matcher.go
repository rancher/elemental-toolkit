package grsync

import (
	"regexp"
)

type matcher struct {
	regExp *regexp.Regexp
}

func (m matcher) Match(data string) bool {
	return m.regExp.Match([]byte(data))
}

func (m matcher) Extract(data string) string {
	const submatchCount = 1
	matches := m.regExp.FindAllStringSubmatch(data, submatchCount)
	if len(matches) == 0 || len(matches[0]) < 2 {
		return ""
	}

	return matches[0][1]
}

func (m matcher) ExtractAllStringSubmatch(data string, submatchCount int) [][]string {
	return m.regExp.FindAllStringSubmatch(data, submatchCount)
}

func newMatcher(regExpString string) *matcher {
	return &matcher{
		regExp: regexp.MustCompile(regExpString),
	}
}
