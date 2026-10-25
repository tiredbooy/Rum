package download

// Machine-readable error codes surfaced next to a job's free-text error. The
// free-text "error" stays for display and logs; clients branch on the code.
const (
	// ErrorCodeDiskFull marks a download stopped because the destination volume
	// has too little free space. The partial file and resume sidecar are kept.
	ErrorCodeDiskFull = "disk_full"
)
