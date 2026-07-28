package ocr

import "errors"

// Shared adapter sentinels (engine-agnostic product mapping uses these via wrapping).
var (
	ErrEmptyImage      = errors.New("ocr: empty image")
	ErrRecognizeFailed = errors.New("ocr: recognize failed")
)
