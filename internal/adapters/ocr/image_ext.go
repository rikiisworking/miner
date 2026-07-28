package ocr

// imageExt picks a temp-file suffix from magic bytes so decoders sniff correctly.
func imageExt(image []byte) string {
	if len(image) >= 8 && string(image[:8]) == "\x89PNG\r\n\x1a\n" {
		return ".png"
	}
	if len(image) >= 3 && image[0] == 0xff && image[1] == 0xd8 && image[2] == 0xff {
		return ".jpg"
	}
	if len(image) >= 6 && (string(image[:6]) == "GIF87a" || string(image[:6]) == "GIF89a") {
		return ".gif"
	}
	if len(image) >= 2 && string(image[:2]) == "BM" {
		return ".bmp"
	}
	if len(image) >= 12 && string(image[:4]) == "RIFF" && string(image[8:12]) == "WEBP" {
		return ".webp"
	}
	return ".img"
}
