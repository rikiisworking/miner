package ocr_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/ocr"
	"github.com/rikiisworking/miner/internal/ports"
)

func TestStatic_ReturnsText(t *testing.T) {
	var eng ports.OcrEngine = ocr.Static{Text: "病院に行った。"}
	got, err := eng.Recognize(context.Background(), []byte("anything"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "病院に行った。" {
		t.Fatalf("got %q", got)
	}
}

func TestStatic_ErrWins(t *testing.T) {
	want := errors.New("boom")
	eng := ocr.Static{Text: "ignored", Err: want}
	_, err := eng.Recognize(context.Background(), []byte("x"))
	if !errors.Is(err, want) {
		t.Fatalf("got %v want %v", err, want)
	}
}

func TestStatic_RespectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ocr.Static{Text: "x"}.Recognize(ctx, []byte("img"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v want context.Canceled", err)
	}
}
