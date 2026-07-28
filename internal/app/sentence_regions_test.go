package app_test

import (
	"context"
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/ocr"
	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/ports"
)

func TestMapLinesToSentenceRegions_TwoSentences(t *testing.T) {
	lines := []ports.OcrLine{
		{Text: "病院に行った。", X: 10, Y: 20, W: 100, H: 30},
		{Text: "私は本を読む。", X: 10, Y: 60, W: 120, H: 30},
	}
	sents := []string{"病院に行った。", "私は本を読む。"}
	got := app.MapLinesToSentenceRegions(lines, sents, 200, 200)
	if len(got) != 2 {
		t.Fatalf("regions=%+v want 2", got)
	}
	if got[0].Text != sents[0] || got[1].Text != sents[1] {
		t.Fatalf("texts=%q %q", got[0].Text, got[1].Text)
	}
	if got[0].X < 0.04 || got[0].X > 0.06 {
		t.Fatalf("x0=%v", got[0].X)
	}
	if got[0].W < 0.45 || got[0].W > 0.55 {
		t.Fatalf("w0=%v", got[0].W)
	}
	if got[1].Y < 0.25 || got[1].Y > 0.35 {
		t.Fatalf("y1=%v", got[1].Y)
	}
}

func TestMapLinesToSentenceRegions_EmptyWithoutGeometry(t *testing.T) {
	if got := app.MapLinesToSentenceRegions(nil, []string{"あ。"}, 100, 100); got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
	if got := app.MapLinesToSentenceRegions([]ports.OcrLine{{Text: "あ。", X: 0, Y: 0, W: 10, H: 10}}, []string{"あ。"}, 0, 100); got != nil {
		t.Fatalf("zero imgW → nil, got %+v", got)
	}
	if got := app.MapLinesToSentenceRegions([]ports.OcrLine{{Text: "あ。", X: 0, Y: 0, W: 10, H: 10}}, []string{"あ。"}, 100, 0); got != nil {
		t.Fatalf("zero imgH → nil, got %+v", got)
	}
	if got := app.MapLinesToSentenceRegions([]ports.OcrLine{{Text: "あ。", X: 0, Y: 0, W: 10, H: 10}}, nil, 100, 100); got != nil {
		t.Fatalf("empty sentences → nil, got %+v", got)
	}
}

func TestMapLinesToSentenceRegions_MultiLineSentence_UnionsBoxes(t *testing.T) {
	lines := []ports.OcrLine{
		{Text: "病院に", X: 10, Y: 10, W: 40, H: 20},
		{Text: "行った。", X: 10, Y: 40, W: 50, H: 20},
	}
	got := app.MapLinesToSentenceRegions(lines, []string{"病院に行った。"}, 100, 100)
	if len(got) != 1 {
		t.Fatalf("regions=%+v want 1", got)
	}
	// Union: x=10..60, y=10..60 → w=50, h=50
	if got[0].X < 0.09 || got[0].X > 0.11 {
		t.Fatalf("x=%v", got[0].X)
	}
	if got[0].W < 0.49 || got[0].W > 0.51 {
		t.Fatalf("w=%v", got[0].W)
	}
	if got[0].H < 0.49 || got[0].H > 0.51 {
		t.Fatalf("h=%v", got[0].H)
	}
}

func TestMapLinesToSentenceRegions_WhitespaceInLines_StillMatches(t *testing.T) {
	lines := []ports.OcrLine{
		{Text: "病 院 に 行 っ た 。", X: 0, Y: 0, W: 80, H: 20},
	}
	got := app.MapLinesToSentenceRegions(lines, []string{"病院に行った。"}, 100, 100)
	if len(got) != 1 {
		t.Fatalf("want match despite OCR spaces: %+v", got)
	}
}

func TestMapLinesToSentenceRegions_Mismatch_SkipsUnplaceable(t *testing.T) {
	lines := []ports.OcrLine{
		{Text: "病院に行った。", X: 0, Y: 0, W: 50, H: 20},
	}
	got := app.MapLinesToSentenceRegions(lines, []string{"病院に行った。", "全然違う文。"}, 100, 100)
	if len(got) != 1 || got[0].Text != "病院に行った。" {
		t.Fatalf("want only placeable: %+v", got)
	}
}

func TestMapLinesToSentenceRegions_TotalMismatch_ReturnsNil(t *testing.T) {
	lines := []ports.OcrLine{
		{Text: "AAA", X: 0, Y: 0, W: 10, H: 10},
	}
	if got := app.MapLinesToSentenceRegions(lines, []string{"病院に行った。"}, 100, 100); got != nil {
		t.Fatalf("want nil: %+v", got)
	}
}

func TestMapLinesToSentenceRegions_ZeroAreaLineBoxes_Skipped(t *testing.T) {
	lines := []ports.OcrLine{
		{Text: "病院に行った。", X: 0, Y: 0, W: 0, H: 20},
	}
	if got := app.MapLinesToSentenceRegions(lines, []string{"病院に行った。"}, 100, 100); got != nil {
		t.Fatalf("zero-area → nil: %+v", got)
	}
}

func TestIngestPage_WithLineGeometry_ProducesRegions(t *testing.T) {
	m := newAppWithOCR(t, ocr.Static{
		Text: "病院に行った。私は本を読む。",
		Lines: []ports.OcrLine{
			{Text: "病院に行った。", X: 0, Y: 0, W: 50, H: 20},
			{Text: "私は本を読む。", X: 0, Y: 30, W: 60, H: 20},
		},
		Width:  100,
		Height: 100,
	})
	got, err := m.IngestPage(context.Background(), []byte("img"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 2 {
		t.Fatalf("candidates=%v", got.Candidates)
	}
	if len(got.Regions) != 2 {
		t.Fatalf("regions=%+v", got.Regions)
	}
	if got.ImgW != 100 || got.ImgH != 100 {
		t.Fatalf("dims %d %d", got.ImgW, got.ImgH)
	}
	for i := range got.Regions {
		if got.Regions[i].Text != got.Candidates[i] {
			t.Fatalf("region%d text mismatch", i)
		}
	}
}

func TestIngestPage_NoLineGeometry_EmptyRegions(t *testing.T) {
	m := newAppWithOCR(t, ocr.Static{Text: "私は本を読む。"})
	got, err := m.IngestPage(context.Background(), []byte("img"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("candidates=%v", got.Candidates)
	}
	if len(got.Regions) != 0 {
		t.Fatalf("want empty regions: %+v", got.Regions)
	}
}

func TestIngestPage_LineGeometryMismatch_StillReturnsCandidates(t *testing.T) {
	m := newAppWithOCR(t, ocr.Static{
		Text:   "私は本を読む。",
		Lines:  []ports.OcrLine{{Text: "全然違う", X: 0, Y: 0, W: 10, H: 10}},
		Width:  100,
		Height: 100,
	})
	got, err := m.IngestPage(context.Background(), []byte("img"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("candidates=%v", got.Candidates)
	}
	if len(got.Regions) != 0 {
		t.Fatalf("mismatch → empty regions: %+v", got.Regions)
	}
}
