package format

import (
	"net/url"
	"path"
	"strings"
)

func GetFolderName(contentType string) string {
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	switch {
	// Archives / Compressed
	case strings.HasPrefix(contentType, "application/x-rar-compressed"),
		strings.HasPrefix(contentType, "application/rar"),
		strings.HasPrefix(contentType, "application/vnd.rar"),
		strings.HasPrefix(contentType, "application/zip"),
		strings.HasPrefix(contentType, "application/x-zip-compressed"),
		strings.HasPrefix(contentType, "application/x-7z-compressed"),
		strings.HasPrefix(contentType, "application/x-7z"),
		strings.HasPrefix(contentType, "application/x-tar"),
		strings.HasPrefix(contentType, "application/gzip"),
		strings.HasPrefix(contentType, "application/x-gzip"),
		strings.HasPrefix(contentType, "application/x-bzip2"),
		strings.HasPrefix(contentType, "application/x-bzip"),
		strings.HasPrefix(contentType, "application/x-xz"),
		strings.HasPrefix(contentType, "application/x-lzma"),
		strings.HasPrefix(contentType, "application/x-compress"),
		strings.HasPrefix(contentType, "application/x-stuffit"),
		strings.HasPrefix(contentType, "application/x-archive"),
		strings.HasPrefix(contentType, "application/x-cpio"),
		strings.HasPrefix(contentType, "application/x-rpm"),
		strings.HasPrefix(contentType, "application/x-deb"),
		strings.HasPrefix(contentType, "application/x-iso9660-image"),
		strings.HasPrefix(contentType, "application/x-apple-diskimage"),
		strings.HasPrefix(contentType, "application/vnd.android.package-archive"),
		strings.HasPrefix(contentType, "application/vnd.apple.installer+xml"):
		return "compressed"

	// Videos
	case strings.HasPrefix(contentType, "video/"):
		return "videos"

	// Audio
	case strings.HasPrefix(contentType, "audio/"):
		return "audios"

	// Images
	case strings.HasPrefix(contentType, "image/"):
		return "images"

	// Documents (PDF, Office, text, etc.)
	case contentType == "application/pdf",
		strings.HasPrefix(contentType, "application/msword"),
		strings.HasPrefix(contentType, "application/vnd.openxmlformats-officedocument.wordprocessingml"),
		strings.HasPrefix(contentType, "application/vnd.openxmlformats-officedocument.presentationml"),
		strings.HasPrefix(contentType, "application/vnd.openxmlformats-officedocument.spreadsheetml"),
		strings.HasPrefix(contentType, "application/vnd.ms-excel"),
		strings.HasPrefix(contentType, "application/vnd.ms-powerpoint"),
		strings.HasPrefix(contentType, "application/vnd.oasis.opendocument.text"),
		strings.HasPrefix(contentType, "application/vnd.oasis.opendocument.spreadsheet"),
		strings.HasPrefix(contentType, "application/vnd.oasis.opendocument.presentation"),
		strings.HasPrefix(contentType, "application/rtf"),
		strings.HasPrefix(contentType, "text/plain"),
		strings.HasPrefix(contentType, "text/csv"),
		strings.HasPrefix(contentType, "text/tab-separated-values"),
		strings.HasPrefix(contentType, "text/markdown"),
		strings.HasPrefix(contentType, "application/x-latex"),
		strings.HasPrefix(contentType, "application/x-tex"),
		strings.HasPrefix(contentType, "application/postscript"):
		return "documents"

	// E-books
	case strings.HasPrefix(contentType, "application/epub+zip"),
		strings.HasPrefix(contentType, "application/x-mobipocket-ebook"),
		strings.HasPrefix(contentType, "application/vnd.amazon.ebook"),
		strings.HasPrefix(contentType, "application/x-kindle-application"),
		strings.HasPrefix(contentType, "application/x-ibooks+zip"),
		strings.HasPrefix(contentType, "application/vnd.comicbook-rar"),
		strings.HasPrefix(contentType, "application/vnd.comicbook+zip"):
		return "ebooks"

	// Programs & executables
	case strings.HasPrefix(contentType, "application/vnd.microsoft.portable-executable"),
		strings.HasPrefix(contentType, "application/x-msdownload"),
		strings.HasPrefix(contentType, "application/x-executable"),
		strings.HasPrefix(contentType, "application/x-mach-binary"),
		strings.HasPrefix(contentType, "application/x-elf"),
		strings.HasPrefix(contentType, "application/x-sh"),
		strings.HasPrefix(contentType, "application/x-shellscript"),
		strings.HasPrefix(contentType, "application/x-python-code"),
		strings.HasPrefix(contentType, "application/x-python-script"),
		strings.HasPrefix(contentType, "application/x-ruby"),
		strings.HasPrefix(contentType, "application/x-perl"),
		strings.HasPrefix(contentType, "text/x-script.python"),
		strings.HasPrefix(contentType, "text/x-python"),
		strings.HasPrefix(contentType, "application/x-msi"),
		strings.HasPrefix(contentType, "application/x-dosexec"):
		return "programs"

	// Fonts
	case strings.HasPrefix(contentType, "font/"),
		strings.HasPrefix(contentType, "application/font-woff"),
		strings.HasPrefix(contentType, "application/font-woff2"),
		strings.HasPrefix(contentType, "application/vnd.ms-fontobject"),
		strings.HasPrefix(contentType, "application/x-font-ttf"),
		strings.HasPrefix(contentType, "application/x-font-otf"):
		return "fonts"

	// Disk images
	case strings.HasPrefix(contentType, "application/x-iso9660-image"),
		strings.HasPrefix(contentType, "application/x-cd-image"),
		strings.HasPrefix(contentType, "application/x-raw-disk-image"),
		strings.HasPrefix(contentType, "application/x-disk-image"),
		strings.HasPrefix(contentType, "application/x-apple-diskimage"),
		strings.HasPrefix(contentType, "application/x-dmg"):
		// Note: some disk images might also be in compressed, but we put them here for clarity.
		return "disk-images"

	// Web & source code
	case strings.HasPrefix(contentType, "text/html"),
		strings.HasPrefix(contentType, "text/css"),
		strings.HasPrefix(contentType, "text/javascript"),
		strings.HasPrefix(contentType, "application/javascript"),
		strings.HasPrefix(contentType, "application/x-javascript"),
		strings.HasPrefix(contentType, "application/json"),
		strings.HasPrefix(contentType, "application/xml"),
		strings.HasPrefix(contentType, "text/xml"),
		strings.HasPrefix(contentType, "application/x-yaml"),
		strings.HasPrefix(contentType, "text/x-csrc"),
		strings.HasPrefix(contentType, "text/x-c++src"),
		strings.HasPrefix(contentType, "text/x-java-source"),
		strings.HasPrefix(contentType, "text/x-csharp"),
		strings.HasPrefix(contentType, "text/x-go"),
		strings.HasPrefix(contentType, "text/x-rust"):
		return "code"

	// Databases & structured data
	case strings.HasPrefix(contentType, "application/sql"),
		strings.HasPrefix(contentType, "application/x-sqlite3"),
		strings.HasPrefix(contentType, "application/vnd.sqlite3"),
		strings.HasPrefix(contentType, "application/x-msaccess"),
		strings.HasPrefix(contentType, "application/vnd.ms-access"),
		strings.HasPrefix(contentType, "application/csv"):
		return "data"

	default:
		return "others"
	}
}

func CleanFileName(url string) string {
	urlArr := strings.Split(url, "?")
	fileName := path.Base(urlArr[0])
	return fileName
}

func ExtractFileNameFromURL(inputUrl string) string {
	parsed, err := url.Parse(inputUrl)
	if err != nil {
		return ""
	}

	if fname := parsed.Query().Get("filename"); fname != "" {
		decoded, err := url.QueryUnescape(fname)
		if err == nil && decoded != "" {
			return decoded
		}

		return fname
	}

	return ""
}
