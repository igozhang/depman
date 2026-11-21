package store

import "errors"

var ErrNotFound = errors.New("not found")

type validationError struct{ msg string }

func (e validationError) Error() string { return e.msg }

func fmtError(msg string) error { return validationError{msg: msg} }

func IsValidation(err error) bool {
	_, ok := err.(validationError)
	return ok
}
