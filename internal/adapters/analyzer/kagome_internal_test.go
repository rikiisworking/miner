package analyzer

import "testing"

func TestKatakanaToHiragana(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"カタカナ", "かたかな"},
		{"ヴァ", "ゔぁ"},
		{"ヵ", "か"},
		{"ヶ", "け"},
		{"ァンヴヵヶ", "ぁんゔかけ"},
		{"ひらがな", "ひらがな"},
		{"A本", "A本"},
		{"", ""},
	}
	for _, tc := range tests {
		got := katakanaToHiragana(tc.in)
		if got != tc.want {
			t.Errorf("katakanaToHiragana(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
