package download

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"
)

func TestClassifyJobError(t *testing.T) {
	dialTimeout := &url.Error{Op: "Get", URL: "https://x.test/f", Err: &net.OpError{
		Op: "dial", Net: "tcp", Err: &fakeTimeout{},
	}}
	tests := []struct {
		name      string
		err       error
		code      string
		retryable bool
	}{
		{"nil", nil, "", false},
		{"dns", &url.Error{Op: "Get", Err: &net.DNSError{Err: "no such host", Name: "x.test", IsNotFound: true}}, ErrCodeDNS, true},
		{"dns timeout is still dns", &net.DNSError{Err: "i/o timeout", Name: "x.test", IsTimeout: true}, ErrCodeDNS, true},
		{"tls unknown authority", fmt.Errorf("get: %w", x509.UnknownAuthorityError{}), ErrCodeTLS, false},
		{"tls handshake text", errors.New("remote error: tls: handshake failure"), ErrCodeTLS, false},
		{"dial timeout", dialTimeout, ErrCodeTimeout, true},
		{"deadline", fmt.Errorf("wrap: %w", context.DeadlineExceeded), ErrCodeTimeout, true},
		{"proxy text", errors.New("proxyconnect tcp: dial tcp 127.0.0.1:1080: connect: connection refused"), ErrCodeProxy, true},
		{"socks text", errors.New("socks connect tcp 1.2.3.4:443->x.test:443: unknown error host unreachable"), ErrCodeProxy, true},
		{"conn refused", &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}, ErrCodeNetworkDown, true},
		{"conn reset", fmt.Errorf("read: %w", syscall.ECONNRESET), ErrCodeNetworkDown, true},
		{"net unreachable", syscall.ENETUNREACH, ErrCodeNetworkDown, true},
		{"unexpected eof", io.ErrUnexpectedEOF, ErrCodeNetworkDown, true},
		{"http 503", &httpStatusError{code: 503}, ErrCodeServerError, true},
		{"http 500", &httpStatusError{code: 500}, ErrCodeServerError, true},
		{"http 429", &httpStatusError{code: 429}, ErrCodeServerError, true},
		{"http 418 unknown", &httpStatusError{code: 418}, ErrCodeUnknown, false},
		{"permission", fmt.Errorf("open: %w", os.ErrPermission), ErrCodePermission, false},
		{"eacces", &os.PathError{Op: "open", Path: "/x", Err: syscall.EACCES}, ErrCodePermission, false},
		{"disk sentinel", fmt.Errorf("x: %w", ErrInsufficientDiskSpace), ErrCodeDiskFull, false},
		{"enospc", &os.PathError{Op: "write", Path: "/x", Err: syscall.ENOSPC}, ErrCodeDiskFull, false},
		{"unknown", errors.New("something odd"), ErrCodeUnknown, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, retry := ClassifyJobError(tc.err)
			if code != tc.code || retry != tc.retryable {
				t.Fatalf("got (%q,%v), want (%q,%v)", code, retry, tc.code, tc.retryable)
			}
		})
	}
}

func TestSetErrorSetsAndClearsCode(t *testing.T) {
	j := &Job{}
	j.SetError(&httpStatusError{code: 502})
	if c, r := j.GetErrorCode(); c != ErrCodeServerError || !r {
		t.Fatalf("got (%q,%v)", c, r)
	}
	j.SetError(nil)
	if c, r := j.GetErrorCode(); c != "" || r {
		t.Fatalf("not cleared: (%q,%v)", c, r)
	}
}

func TestJobErrorCodePersistsAndBackfills(t *testing.T) {
	j := &Job{ID: "a"}
	j.SetError(context.DeadlineExceeded)
	b, err := j.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Job
	if err := back.UnmarshalJSON(b); err != nil {
		t.Fatal(err)
	}
	if c, _ := back.GetErrorCode(); c != ErrCodeTimeout {
		t.Fatalf("roundtrip code = %q", c)
	}
	// Legacy file: error text, no code -> classified on load.
	var old Job
	if err := old.UnmarshalJSON([]byte(`{"id":"b","status":"error","error":"no such host"}`)); err != nil {
		t.Fatal(err)
	}
	if c, _ := old.GetErrorCode(); c == "" {
		t.Fatal("legacy error not backfilled")
	}
}

type fakeTimeout struct{}

func (*fakeTimeout) Error() string   { return "i/o timeout" }
func (*fakeTimeout) Timeout() bool   { return true }
func (*fakeTimeout) Temporary() bool { return true }
