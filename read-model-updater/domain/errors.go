package domain

import "errors"

// ErrConcurrencyConflict indicates that the underlying storage rejected an
// update because a newer version of the entity is already persisted.
var ErrConcurrencyConflict = errors.New("concurrency conflict")
