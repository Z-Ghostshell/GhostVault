package api

import "testing"

func TestBuildRetrieveMeta(t *testing.T) {
	t.Parallel()
	m0 := buildRetrieveMeta(0)
	if m0["result_count"] != 0 {
		t.Fatalf("result_count: %v", m0["result_count"])
	}
	if _, ok := m0["hint"]; !ok {
		t.Fatal("expected hint when result_count is 0")
	}
	m3 := buildRetrieveMeta(3)
	if m3["result_count"] != 3 {
		t.Fatalf("result_count: %v", m3["result_count"])
	}
	if _, ok := m3["hint"]; ok {
		t.Fatal("expected no hint when result_count > 0")
	}
}
