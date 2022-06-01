package db

import "errors"

var ErrNotFound = errors.New("not found")

func IgnoreErrNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
