package rag

import "testing"

func TestKeywordScoreExactMatch(t *testing.T) {
	score := KeywordScore("阿司匹林", "阿司匹林是一种常用的解热镇痛药")
	if score <= 0 {
		t.Errorf("expected positive score for matching keywords, got %.4f", score)
	}
	t.Logf("score for '阿司匹林': %.4f", score)
}

func TestKeywordScoreNoMatch(t *testing.T) {
	score := KeywordScore("青霉素", "阿司匹林是一种常用的解热镇痛药")
	if score != 0 {
		t.Errorf("expected 0 for no match, got %.4f", score)
	}
}

func TestKeywordScoreEmpty(t *testing.T) {
	if s := KeywordScore("", "test"); s != 0 {
		t.Errorf("expected 0 for empty query, got %.4f", s)
	}
	if s := KeywordScore("test", ""); s != 0 {
		t.Errorf("expected 0 for empty doc, got %.4f", s)
	}
}

func TestKeywordScorePartialMatch(t *testing.T) {
	// "副作用" and "胃肠道" should match
	score1 := KeywordScore("副作用胃肠道", "常见的副作用包括胃肠道反应：胃痛、恶心")
	score2 := KeywordScore("副作用胃肠道", "阿司匹林是解热镇痛药用于退烧止痛")
	if score1 <= score2 {
		t.Errorf("expected higher score for better match: %.4f vs %.4f", score1, score2)
	}
	t.Logf("good match: %.4f, bad match: %.4f", score1, score2)
}

func TestKeywordScoreStopWords(t *testing.T) {
	// Stop words like 的, 了, 是 should be filtered out.
	score := KeywordScore("的了吗是", "测试文本")
	if score != 0 {
		t.Errorf("expected 0 for stop-word-only query, got %.4f", score)
	}
}
