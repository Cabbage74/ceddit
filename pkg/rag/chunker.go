package rag

import (
	"fmt"
	"strings"
)

const (
	// DefaultChunkSize is the maximum number of characters per chunk.
	DefaultChunkSize = 800
	// DefaultChunkOverlap is the number of overlapping characters between adjacent chunks.
	DefaultChunkOverlap = 100
)

// ChunkMarkdown splits markdown text into chunks:
// 1. Split by markdown headings (lines starting with #).
// 2. Further split long paragraphs into fixed-size chunks with overlap.
func ChunkMarkdown(text string) []string {
	return ChunkMarkdownWithOptions(text, DefaultChunkSize, DefaultChunkOverlap)
}

// ChunkMarkdownWithOptions splits markdown text with custom chunk size and overlap.
func ChunkMarkdownWithOptions(text string, maxChars, overlap int) []string {
	paras := splitByHeadings(text)
	return fixedChunks(paras, maxChars, overlap)
}

// ExtractTitle returns the first H1 heading found in the text, or empty string.
func ExtractTitle(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
	}
	return ""
}

// splitByHeadings splits text into paragraphs at markdown heading boundaries.
func splitByHeadings(text string) []string {
	var paras []string
	lines := strings.Split(text, "\n")
	var buf strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isHeader := strings.HasPrefix(trimmed, "#")

		if isHeader && buf.Len() > 0 {
			paras = append(paras, buf.String())
			buf.Reset()
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	if buf.Len() > 0 {
		paras = append(paras, buf.String())
	}

	return paras
}

// fixedChunks splits paragraphs into fixed-size chunks with overlap.
func fixedChunks(paras []string, maxChars, overlap int) []string {
	var chunks []string

	for _, p := range paras {
		runes := []rune(p)
		if len(runes) <= maxChars {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				chunks = append(chunks, trimmed)
			}
		} else {
			start := 0
			for start < len(runes) {
				end := start + maxChars
				if end > len(runes) {
					end = len(runes)
				}

				chunk := strings.TrimSpace(string(runes[start:end]))
				if chunk != "" {
					chunks = append(chunks, chunk)
				}

				if end >= len(runes) {
					break
				}

				// Overlap to preserve semantic continuity.
				nextStart := end - overlap
				if nextStart <= start {
					nextStart = start + 1 // ensure forward progress
				}
				start = nextStart
			}
		}
	}

	return chunks
}

// NearestHeading finds the nearest heading above a given chunk position.
func NearestHeading(headings []string, chunkIdx, totalChunks int) string {
	if len(headings) == 0 {
		return ""
	}
	// Map chunk position back to approximate heading index.
	headingIdx := chunkIdx * len(headings) / totalChunks
	if headingIdx >= len(headings) {
		headingIdx = len(headings) - 1
	}
	return headings[headingIdx]
}

// CollectHeadings extracts all heading lines from markdown text.
func CollectHeadings(text string) []string {
	var headings []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			headings = append(headings, strings.TrimLeft(trimmed, "# "))
		}
	}
	return headings
}

// FormatChunkID creates a chunk identifier like "postID#position".
func FormatChunkID(postID int64, position int) string {
	return fmt.Sprintf("%d#%d", postID, position)
}
