//go:build unix

package download

import (
	"errors"

	"golang.org/x/sys/unix"
)

func isNoSpaceErrno(err error) bool {
	return errors.Is(err, unix.ENOSPC) || errors.Is(err, unix.EDQUOT)
}
