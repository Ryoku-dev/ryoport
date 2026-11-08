package palette

import (
	"sort"
	"strings"
	"unicode"
)

type rankItem struct {
	Text     string
	Resource bool
}

type rankedItem struct {
	index int
	score int
}

func rank(query string, items []rankItem) []int {
	query = strings.TrimSpace(query)
	if query == "" {
		indices := make([]int, 0, len(items))
		for i := range items {
			if items[i].Resource {
				indices = append(indices, i)
			}
		}
		for i := range items {
			if !items[i].Resource {
				indices = append(indices, i)
			}
		}
		return indices
	}

	matches := make([]rankedItem, 0, len(items))
	for i, item := range items {
		if score, ok := fuzzyScore(query, item.Text); ok {
			matches = append(matches, rankedItem{index: i, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})
	indices := make([]int, len(matches))
	for i, match := range matches {
		indices[i] = match.index
	}
	return indices
}

func fuzzyScore(query, text string) (int, bool) {
	needle := []rune(strings.ToLower(strings.TrimSpace(query)))
	haystack := []rune(strings.ToLower(text))
	if len(needle) == 0 {
		return 0, true
	}
	if len(needle) > len(haystack) {
		return 0, false
	}

	if equalRunes(needle, haystack) {
		return 5000, true
	}
	contiguous := -1
	for start := 0; start+len(needle) <= len(haystack); start++ {
		if !equalRunes(needle, haystack[start:start+len(needle)]) {
			continue
		}
		extra := len(haystack) - len(needle)
		score := 2000 - min(start+extra, 500)
		switch {
		case start == 0:
			score = 4000 - min(extra, 500)
		case wordStart(haystack, start):
			score = 3000 - min(start+extra, 500)
		}
		contiguous = max(contiguous, score)
	}
	if contiguous >= 0 {
		return contiguous, true
	}

	best := -1
	for start, r := range haystack {
		if r != needle[0] {
			continue
		}
		at := start
		adjacent, boundaries, gaps := 0, 0, 0
		if wordStart(haystack, start) {
			boundaries++
		}
		matched := true
		for _, want := range needle[1:] {
			next := at + 1
			for next < len(haystack) && haystack[next] != want {
				next++
			}
			if next == len(haystack) {
				matched = false
				break
			}
			if next == at+1 {
				adjacent++
			} else {
				gaps += next - at - 1
			}
			if wordStart(haystack, next) {
				boundaries++
			}
			at = next
		}
		if !matched {
			continue
		}
		bonus := min(adjacent*12+boundaries*18, 400)
		penalty := min(start+gaps*4, 700)
		score := 1000 + bonus - penalty
		if score > best {
			best = score
		}
	}
	return best, best >= 0
}

func equalRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func wordStart(text []rune, at int) bool {
	return at == 0 || !unicode.IsLetter(text[at-1]) && !unicode.IsNumber(text[at-1])
}
