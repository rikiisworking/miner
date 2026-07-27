package analyzer_test

import (
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/analyzer"
)

func TestStub_FixtureHasContentAndParticles(t *testing.T) {
	toks, err := analyzer.Stub{}.Analyze("私は本を読む。")
	if err != nil {
		t.Fatal(err)
	}
	var content, non int
	for _, tok := range toks {
		if tok.Content {
			content++
		} else {
			non++
		}
	}
	if content != 3 {
		t.Fatalf("content count=%d want 3", content)
	}
	if non != 3 {
		t.Fatalf("non-content count=%d want 3 (particles+punct)", non)
	}
}

func TestStub_ForceError(t *testing.T) {
	_, err := analyzer.Stub{}.Analyze(analyzer.ForceErrorText)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStub_FallbackWholeSurface(t *testing.T) {
	toks, err := analyzer.Stub{}.Analyze("未知の文")
	if err != nil {
		t.Fatal(err)
	}
	if len(toks) != 1 || toks[0].Surface != "未知の文" || !toks[0].Content {
		t.Fatalf("unexpected fallback: %+v", toks)
	}
}
