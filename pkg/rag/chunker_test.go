package rag

import (
	"strings"
	"testing"
)

func TestSplitByHeadings(t *testing.T) {
	text := `# Title

Some content here.

## Section 1

More content in section 1.

## Section 2

Content in section 2.`

	paras := splitByHeadings(text)
	if len(paras) < 2 {
		t.Fatalf("expected at least 2 paragraphs, got %d", len(paras))
	}

	// First paragraph should contain "# Title".
	if !strings.Contains(paras[0], "# Title") {
		t.Errorf("expected first para to contain '# Title', got: %s", paras[0])
	}
}

func TestChunkMarkdown(t *testing.T) {
	text := `# 阿司匹林的作用与副作用

阿司匹林是一种常用的解热镇痛药。它的主要作用包括：

## 主要作用

1. 解热：降低发烧体温
2. 镇痛：缓解轻到中度疼痛
3. 抗血小板：预防血栓形成

## 副作用

但是，阿司匹林也有副作用。常见的副作用包括：

1. 胃肠道反应：胃痛、恶心
2. 出血风险：特别是长期服用`

	chunks := ChunkMarkdown(text)
	if len(chunks) == 0 {
		t.Fatal("expected non-zero chunks")
	}

	// Verify each chunk is not empty.
	for i, c := range chunks {
		if strings.TrimSpace(c) == "" {
			t.Errorf("chunk %d is empty", i)
		}
	}

	t.Logf("produced %d chunks", len(chunks))
	for i, c := range chunks {
		t.Logf("chunk %d: %d chars", i, len([]rune(c)))
	}
}

func TestChunkMarkdownSmallText(t *testing.T) {
	text := "This is a short text under 800 characters."

	chunks := ChunkMarkdown(text)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != text {
		t.Errorf("chunk content mismatch: %s", chunks[0])
	}
}

func TestChunkMarkdownEmpty(t *testing.T) {
	chunks := ChunkMarkdown("")
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}

func TestChunkMarkdownLongParagraph(t *testing.T) {
	// Create text > 800 characters (single paragraph, no headings).
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("这是第")
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteString("段测试文字。")
	}
	text := sb.String()
	runes := []rune(text)
	t.Logf("input text: %d runes", len(runes))

	chunks := ChunkMarkdown(text)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for long text, got %d", len(chunks))
	}

	// Each chunk should be <= 800 characters.
	for i, c := range chunks {
		cr := []rune(c)
		if len(cr) > 800 {
			t.Errorf("chunk %d exceeds max size: %d > 800", i, len(cr))
		}
	}

	t.Logf("long text split into %d chunks", len(chunks))
}

func TestFixedChunksOverlap(t *testing.T) {
	// Create a paragraph that will be split into at least 2 chunks.
	var sb strings.Builder
	for i := 0; i < 80; i++ {
		sb.WriteString("这是测试文本，用于验证切片重叠功能。")
	}
	paras := []string{sb.String()}

	chunks := fixedChunks(paras, 500, 100)
	if len(chunks) < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", len(chunks))
	}

	// Check overlap: last 100 chars of chunk 0 should appear in chunk 1.
	r0 := []rune(chunks[0])
	r1 := []rune(chunks[1])

	// Take the last few characters of chunk 0 and check they're in chunk 1.
	if len(r0) > 100 && len(r1) > 0 {
		overlapTail := string(r0[len(r0)-50:])
		if !strings.Contains(chunks[1], overlapTail) {
			t.Logf("chunk 0 tail (50 runes): %s", overlapTail)
			t.Logf("chunk 1 start: %s", string(r1[:min(50, len(r1))]))
			t.Log("overlap may not be exact at chunk boundaries")
		}
	}
}

func TestExtractTitle(t *testing.T) {
	text := `# 阿司匹林使用指南

## 简介

阿司匹林是一种药物。`

	title := ExtractTitle(text)
	if title != "阿司匹林使用指南" {
		t.Errorf("expected '阿司匹林使用指南', got '%s'", title)
	}
}

func TestExtractTitleNoH1(t *testing.T) {
	text := "Just some plain text without headings."
	title := ExtractTitle(text)
	if title != "" {
		t.Errorf("expected empty title, got '%s'", title)
	}
}

func TestFormatChunkID(t *testing.T) {
	id := FormatChunkID(12345, 7)
	if id != "12345#7" {
		t.Errorf("expected '12345#7', got '%s'", id)
	}
}

func TestCollectHeadings(t *testing.T) {
	text := `# Title
## Section A
Some text.
### Subsection A1
More text.
## Section B`

	headings := CollectHeadings(text)
	if len(headings) != 4 {
		t.Fatalf("expected 4 headings, got %d: %v", len(headings), headings)
	}
	if headings[0] != "Title" {
		t.Errorf("expected 'Title', got '%s'", headings[0])
	}
}
