package service

import "errors"

// ErrValidation indicates invalid input from the client.
var ErrValidation = errors.New("validation error")
