package core

import (
	"strings"
	"testing"
)

func TestParseChunkPlaintext_v1(t *testing.T) {
	c := ParseChunkPlaintext("hello world")
	if c.Body != "hello world" || c.SchemaVer != 1 {
		t.Fatalf("v1: %+v", c)
	}
	if c.EmbedSource() != "hello world" {
		t.Fatal()
	}
}

func TestParseChunkPlaintext_v2(t *testing.T) {
	j := `{"v":2,"a":"short","b":"long body here","k":"note","m":{"t":1}}`
	c := ParseChunkPlaintext(j)
	if c.Abstract != "short" || c.Body != "long body here" || c.Kind != "note" || c.SchemaVer != 2 {
		t.Fatalf("v2: %+v", c)
	}
	if c.EmbedSource() != "short" {
		t.Fatal()
	}
	if c.LexicalSource() == "" {
		t.Fatal()
	}
}

func TestEstimatedReturnTokens_both(t *testing.T) {
	c := ChunkBody{Abstract: "a", Body: "bbbb"}
	if c.EstimatedReturnTokens("both") < 1 {
		t.Fatal()
	}
}

func TestExcerptRunes(t *testing.T) {
	s := strings.Repeat("x", 10) + "日本語"
	if ExcerptRunes(s, 8) == s {
		t.Fatal("expected truncation")
	}
}
