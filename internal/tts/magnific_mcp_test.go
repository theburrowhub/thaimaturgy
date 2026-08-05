package tts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMagnificMCPGeneratorRunsCommandAndCachesAudio(t *testing.T) {
	dir := t.TempDir()
	cmdPath := filepath.Join(dir, "fake_magnific_tts.py")
	logPath := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(cmdPath, []byte(`#!/usr/bin/env python3
import json, pathlib, sys
payload=json.load(sys.stdin)
pathlib.Path(payload["debugLogPath"]).write_text(json.dumps(payload, ensure_ascii=False))
pathlib.Path(payload["outputPath"]).write_bytes(b"mp3 data")
print(json.dumps({"audioPath": payload["outputPath"]}))
`), 0700); err != nil {
		t.Fatal(err)
	}
	gen := NewMagnificMCPGenerator(MagnificMCPConfig{
		Command:         "python3 " + cmdPath,
		CacheDir:        filepath.Join(dir, "cache"),
		VoiceID:         467,
		Model:           "eleven_v3",
		Stability:       0.15,
		SimilarityBoost: 0.35,
		Speed:           0.95,
		UseSpeakerBoost: true,
		DebugLogPath:    logPath,
	})

	first, err := gen.Generate(context.Background(), "La puerta se abre.")
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(first); err != nil || string(b) != "mp3 data" {
		t.Fatalf("audio %q = %q, %v", first, b, err)
	}
	payloadBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["text"] != "La puerta se abre." || payload["model"] != "eleven_v3" || payload["voiceId"].(float64) != 467 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload["stability"].(float64) != 0.15 {
		t.Fatalf("stability = %v", payload["stability"])
	}

	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	second, err := gen.Generate(context.Background(), "La puerta se abre.")
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("cache path changed: %s vs %s", second, first)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("command ran on cache hit; log stat err=%v", err)
	}
}
