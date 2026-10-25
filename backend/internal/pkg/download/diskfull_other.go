//go:build !unix && !windows

package download

func isNoSpaceErrno(error) bool { return false }
