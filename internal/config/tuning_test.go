package config

import (
	"encoding/json"
	"testing"
)

func TestMergeTuningFull_fileThenDB(t *testing.T) {
	def := DefaultRuntimeTuning()
	db := []byte(`{"semantic_threshold":0.99}`)
	// no file
	eff, fileL, err := MergeTuningFull(def, "", db)
	if err != nil {
		t.Fatal(err)
	}
	if eff.SemanticThreshold != 0.99 {
		t.Fatalf("db override: got %v", eff.SemanticThreshold)
	}
	if fileL.SemanticThreshold != def.SemanticThreshold {
		t.Fatalf("file layer without file should match def")
	}
	src := ComputeSources(def, fileL, eff)
	if src["semantic_threshold"] != "db" {
		t.Fatalf("source: %s", src["semantic_threshold"])
	}
}

func TestMergeTuningOverrideJSON_invalidKey(t *testing.T) {
	_, err := MergeTuningOverrideJSON([]byte("{}"), map[string]json.RawMessage{
		"embedding_model": []byte(`"x"`),
	})
	if err == nil {
		t.Fatal("expected error for embedding_model in DB patch")
	}
}

func TestMergeFromJSONMap_unknownKey(t *testing.T) {
	_, err := MergeFromJSONMap(DefaultRuntimeTuning(), map[string]json.RawMessage{
		"not_a_key": []byte(`1`),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
