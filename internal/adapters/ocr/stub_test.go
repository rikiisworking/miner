package ocr_test

import (
	"errors"
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/ocr"
)

func TestStub_NotConfigured(t *testing.T) {
	_, err := ocr.Stub{}.Recognize([]byte("img"))
	if !errors.Is(err, ocr.ErrNotConfigured) {
		t.Fatalf("got %v want ErrNotConfigured", err)
	}
}

func TestStub_TextIgnoresImage(t *testing.T) {
	s := ocr.Stub{Text: "病院に行った。"}
	got, err := s.Recognize([]byte("anything"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "病院に行った。" {
		t.Fatalf("got %q", got)
	}
}

func TestStub_ByBytesPreferredOverText(t *testing.T) {
	img := []byte("fixture-a")
	s := ocr.Stub{
		Text:    "fallback",
		ByBytes: map[string]string{string(img): "from-map"},
	}
	got, err := s.Recognize(img)
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-map" {
		t.Fatalf("got %q want from-map", got)
	}
	// Unknown payload falls through to Text.
	got, err = s.Recognize([]byte("other"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "fallback" {
		t.Fatalf("got %q want fallback", got)
	}
}

func TestStub_FailWithWins(t *testing.T) {
	boom := errors.New("engine boom")
	s := ocr.Stub{Text: "ok", FailWith: boom}
	_, err := s.Recognize([]byte("x"))
	if !errors.Is(err, boom) {
		t.Fatalf("got %v want boom", err)
	}
}
