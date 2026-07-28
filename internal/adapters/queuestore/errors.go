package queuestore

import "errors"

// Shared sentinel errors for File and Mem adapters (errors.Is friendly).
var (
	ErrEmptyID     = errors.New("queuestore: empty entry id")
	ErrDuplicateID = errors.New("queuestore: duplicate id")
)
