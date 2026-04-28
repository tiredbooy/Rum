package download

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"time"
)

type HeaderInfo struct {
	ContentSize   string
	ContentType   string
	SupportsRange bool
}

type Downloader struct {
	Client  *http.Client
	Headers map[string]string
}

type RequestHeaders struct {
	Referer        string
	UserAgent      string
	AcceptLanguage string
	AcceptEncoding string
	Connection     string
}

func NewDownloader(userAgent, referer string) *Downloader {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxConnsPerHost:     2,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	return &Downloader{
		Client: client,
		Headers: map[string]string{
			"User-Agent":      userAgent,
			"Referer":         referer,
			"Accept":          "*/*",
			"Accept-Language": "en-US,en;q=0.5",
			"Accept-Encoding": "gzip, deflate, br",
			"Connection":      "keep-alive",
		},
	}
}

func (d *Downloader) NewRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range d.Headers {
		req.Header.Set(key, value)
	}
	return req, nil
}

func (d *Downloader) GetFileRequest(url string) (*http.Request, error) {
	return d.NewRequest("GET", url)
}

func GetHeader(url string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		log.Printf("Failed to create HEAD request for %s: %v", url, err)
		return nil, err
	}

	client := GetHTTPClient(15 * time.Second)
	return client.Do(req)
}

func (d *Downloader) HeadWithFallback(url string) (*HeaderInfo, error) {
	// Try HEAD first
	headReq, _ := d.NewRequest("HEAD", url)
	resp, err := d.Client.Do(headReq)
	if err == nil && resp.StatusCode < 400 {
		defer resp.Body.Close()
		return ParseHeaderInfo(resp), nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Fallback: GET with Range: bytes=0-0
	req, _ := d.NewRequest("GET", url)
	req.Header.Set("Range", "bytes=0-0")
	resp, err = d.Client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusOK {
			return ParseHeaderInfo(resp), nil
		}
		return ParseHeaderInfo(resp), nil
	}

	return &HeaderInfo{}, nil
}

func GetHeaderInfo(url string) (*HeaderInfo, error) {
	headResp, err := GetWithTimeout(url, "HEAD", 10*time.Second)
	if err == nil && headResp != nil {
		defer headResp.Body.Close()
		return ParseHeaderInfo(headResp), nil
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Range", "bytes=0.0")

	client := GetHTTPClient(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HEAD and GET fallback both failed: %s", err)
	}

	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)

	return ParseHeaderInfo(resp), nil
}

func GetHTTPClient(timeout time.Duration) *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: timeout,
		Jar:     jar,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxConnsPerHost:     2,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func ParseHeaderInfo(resp *http.Response) *HeaderInfo {
	acceptRange := resp.Header.Get("Accept-Ranges")
	return &HeaderInfo{
		ContentSize:   resp.Header.Get("Content-Length"),
		ContentType:   resp.Header.Get("Content-Type"),
		SupportsRange: acceptRange != "",
	}
}

func GetWithTimeout(url, method string, timeout time.Duration) (*http.Response, error) {
	req, _ := http.NewRequest(method, url, nil)
	client := &http.Client{Timeout: timeout}
	return client.Do(req)
}
