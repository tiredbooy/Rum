package download

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

type HeaderInfo struct {
	ContentSize   string
	ContentType   string
	SupportsRange bool
}

func GetFile(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return req, nil
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

// func GetHeaderInfo(url string) (*HeaderInfo, error) {
// 	headResp, err := GetHeader(url)
// 	if err != nil {
// 		return nil, err
// 	}

// 	defer headResp.Body.Close()

// 	acceptRange := headResp.Header.Get("Accept-Ranges")

// 	return &HeaderInfo{
// 		ContentSize:   headResp.Header.Get("Content-Length"),
// 		ContentType:   headResp.Header.Get("Content-Type"),
// 		SupportsRange: acceptRange != "",
// 	}, nil
// }

func GetHeaderInfo(url string) (*HeaderInfo, error) {
	headResp, err := GetWithTimeout(url, "HEAD", 10*time.Second)
	if err == nil && headResp != nil {
		defer headResp.Body.Close()
		log.Println("WE GOT THE HEADER FIRST TRY")
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

	log.Println("WE GOT THE HEADER WITH FALLBACK")

	io.Copy(io.Discard, resp.Body)

	return ParseHeaderInfo(resp), nil
}

func GetHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
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

// resp, err := http.DefaultClient.Do(req)
// if err != nil {
// 	log.Printf("Failed to perform HEAD request for %s: %v", url, err)
// 	return nil, err
// }

// return resp, nil
