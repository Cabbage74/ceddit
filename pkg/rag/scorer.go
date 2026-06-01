package rag

import "strings"

// stopChars contains punctuation and common words with low semantic value.
// Using a string-based check to avoid Go rune-literal issues with Unicode quotes.
var stopChars = map[rune]bool{}

func init() {
	for _, r := range " 的了吗呢啊哦吧一不是我在有和就人都会很要到说去看这那你么着好没自" +
		"\t\n\r" +
		".,!?;，。！？；：（）【】《》" {
		stopChars[r] = true
	}
}

// KeywordScore computes a simple TF-overlap score between a query and document.
// Returns a value in [0, 1]. Higher means more query characters appear in the doc.
// Works for both Chinese (character-level) and English (word-level).
func KeywordScore(query, doc string) float64 {
	if query == "" || doc == "" {
		return 0
	}

	queryChars := extractKeywords(query)
	if len(queryChars) == 0 {
		return 0
	}

	docLower := strings.ToLower(doc)
	matched := 0
	for _, ch := range queryChars {
		if strings.ContainsRune(docLower, ch) {
			matched++
		}
	}

	return float64(matched) / float64(len(queryChars))
}

// extractKeywords extracts unique meaningful characters from text.
func extractKeywords(text string) []rune {
	seen := make(map[rune]bool)
	var chars []rune

	for _, ch := range strings.ToLower(text) {
		if stopChars[ch] {
			continue
		}
		if seen[ch] {
			continue
		}
		seen[ch] = true
		chars = append(chars, ch)
	}

	return chars
}
