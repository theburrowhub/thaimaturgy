package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

func TestGenerateSpeechFileWithClientWritesMP3AndRequest(t *testing.T) {
	var gotAuth string
	var gotReq fileSpeechRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3-data"))
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "speech.mp3")
	cfg := &domain.TTSConfig{Enabled: true, Model: "tts-test", Voice: domain.TTSVoiceNova, Speed: 1.25}
	if err := GenerateSpeechFileWithClient(context.Background(), srv.Client(), srv.URL, "test-key", cfg, "Hello players", out); err != nil {
		t.Fatalf("GenerateSpeechFileWithClient: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "mp3-data" {
		t.Fatalf("output = %q", data)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotReq.Model != "tts-test" || gotReq.Voice != "nova" || gotReq.Input != "Hello players" || gotReq.ResponseFormat != "mp3" {
		t.Fatalf("request = %+v", gotReq)
	}
}
