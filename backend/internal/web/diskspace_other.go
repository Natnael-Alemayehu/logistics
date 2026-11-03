//go:build !unix

package web

import "errors"

// diskSpace isn't measured off Unix; the health page then leaves it out.
func diskSpace(string) (uint64, uint64, error) {
	return 0, 0, errors.New("disk space is only measured on Unix")
}
