package download

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
)

// User-facing job error codes (Job.ErrorCode / the DTO "error_code" field).
// They are part of the API contract with the frontend: add, don't repurpose.
const (
	ErrCodeDiskFull    = "disk_full"
	ErrCodeNetworkDown = "network_down"
	ErrCodeDNS         = "dns"
	ErrCodeTLS         = "tls"
	ErrCodeTimeout     = "timeout"
	ErrCodeProxy       = "proxy"
	ErrCodeServerError = "server_error"
	ErrCodePermission  = "permission"
	ErrCodeUnknown     = "unknown"
)

// ClassifyJobError maps a download failure to a user-facing error code and
// whether retrying later could plausibly succeed. nil returns ("", false).
func ClassifyJobError(err error) (code string, retryable bool) {
	if err == nil {
		return "", false
	}
	msg := strings.ToLower(err.Error())

	var dnsErr *net.DNSError
	var unknownAuth x509.UnknownAuthorityError
	var certInvalid x509.CertificateInvalidError
	var hostErr x509.HostnameError
	var verifyErr *tls.CertificateVerificationError
	var recHdr tls.RecordHeaderError
	var se *httpStatusError
	var netErr net.Error

	switch {
	case errors.Is(err, errStalled):
		return ErrorCodeStalled, true
	case errors.Is(err, ErrInsufficientDiskSpace), errors.Is(err, syscall.ENOSPC):
		return ErrCodeDiskFull, false
	case errors.Is(err, os.ErrPermission), errors.Is(err, syscall.EACCES),
		errors.Is(err, syscall.EPERM), errors.Is(err, syscall.EROFS):
		return ErrCodePermission, false
	case errors.As(err, &dnsErr), strings.Contains(msg, "no such host"):
		return ErrCodeDNS, true
	case errors.As(err, &unknownAuth), errors.As(err, &certInvalid), errors.As(err, &hostErr),
		errors.As(err, &verifyErr), errors.As(err, &recHdr),
		strings.Contains(msg, "tls:"), strings.Contains(msg, "x509:"):
		return ErrCodeTLS, false
	case strings.Contains(msg, "proxyconnect"), strings.Contains(msg, "socks"),
		strings.Contains(msg, "proxy"):
		return ErrCodeProxy, true
	case errors.As(err, &se):
		if se.code >= 500 && se.code <= 599 || se.code == http.StatusTooManyRequests ||
			se.code == http.StatusRequestTimeout {
			return ErrCodeServerError, true
		}
		return ErrCodeUnknown, false
	case errors.Is(err, context.DeadlineExceeded),
		errors.As(err, &netErr) && netErr.Timeout(),
		strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "timeout"):
		return ErrCodeTimeout, true
	case errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, syscall.ECONNRESET),
		errors.Is(err, syscall.ECONNABORTED), errors.Is(err, syscall.ENETUNREACH),
		errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETDOWN),
		errors.Is(err, syscall.EPIPE), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF),
		errors.As(err, &netErr),
		strings.Contains(msg, "connection reset"), strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "network is unreachable"), strings.Contains(msg, "no route to host"),
		strings.Contains(msg, "broken pipe"), strings.Contains(msg, "unexpected eof"):
		return ErrCodeNetworkDown, true
	}
	return ErrCodeUnknown, false
}

// ErrorCodeDiskFull is the name the disk-full code carries in the disk-full
// helpers; it is the same value as ErrCodeDiskFull.
const ErrorCodeDiskFull = ErrCodeDiskFull

// ErrorCodeStalled marks a connection that stayed open but delivered no bytes
// for the stall timeout; the engine reconnects and resumes from the offset. It
// is also the final code if the stall outlasts every retry.
const ErrorCodeStalled = "stalled"
