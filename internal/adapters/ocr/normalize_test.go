package ocr

import "testing"

func TestImageExt(t *testing.T) {
	if imageExt([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) != ".png" {
		t.Fatal("png")
	}
	if imageExt([]byte{0xff, 0xd8, 0xff, 0xe0}) != ".jpg" {
		t.Fatal("jpg")
	}
	if imageExt([]byte("GIF89a")) != ".gif" {
		t.Fatal("gif")
	}
	if imageExt([]byte{'B', 'M', 0, 0}) != ".bmp" {
		t.Fatal("bmp")
	}
	webp := make([]byte, 12)
	copy(webp, "RIFF")
	copy(webp[8:], "WEBP")
	if imageExt(webp) != ".webp" {
		t.Fatal("webp")
	}
	if imageExt([]byte{1, 2, 3}) != ".img" {
		t.Fatal("unknown")
	}
}
