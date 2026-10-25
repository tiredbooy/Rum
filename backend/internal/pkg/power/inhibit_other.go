//go:build !linux && !windows && !darwin

package power

type unsupportedBackend struct{}

func (unsupportedBackend) Acquire(string) (func(), error) { return nil, ErrUnsupported }

func systemBackend() Backend { return unsupportedBackend{} }
