package download

import (
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ExportVersion is the schema version of the download-list export file.
const ExportVersion = 1

// ExportedDownload is one entry of the exported list. It deliberately omits
// everything that can carry a secret (referer, user agent, cookies, tokens) and
// all runtime state; URLs are exported as the user entered them.
type ExportedDownload struct {
	URL          string `json:"url"`
	FileName     string `json:"file_name,omitempty"`
	DestPath     string `json:"dest_path,omitempty"`
	Category     string `json:"category,omitempty"`
	Checksum     string `json:"checksum,omitempty"`
	ChecksumAlgo string `json:"checksum_algo,omitempty"`
	SpeedLimit   int    `json:"speed_limit,omitempty"`
	MaxRetries   int    `json:"max_retries,omitempty"`
	// Status is informational: import ignores it except to skip finished items.
	Status string `json:"status,omitempty"`
}

// ExportDoc is the download-list export file.
type ExportDoc struct {
	Version    int                `json:"version"`
	ExportedAt time.Time          `json:"exported_at"`
	Downloads  []ExportedDownload `json:"downloads"`
}

// ImportResult counts what an import did with the items it was given.
type ImportResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"` // duplicates and already-completed items
	Invalid int `json:"invalid"` // bad URL or destination
}

// ExportDownloads snapshots the download list for backup or transfer.
func (m *JobManager) ExportDownloads() ExportDoc {
	jobs := m.GetAllJobs()
	doc := ExportDoc{Version: ExportVersion, ExportedAt: time.Now().UTC(), Downloads: make([]ExportedDownload, 0, len(jobs))}
	for _, j := range jobs {
		j.Mu.RLock()
		doc.Downloads = append(doc.Downloads, ExportedDownload{
			URL: j.URL, FileName: j.FileName, DestPath: j.DestPath, Category: j.Category,
			Checksum: j.Checksum, ChecksumAlgo: j.ChecksumAlgo,
			SpeedLimit: j.SpeedLimit, MaxRetries: j.MaxRetries, Status: j.Status,
		})
		j.Mu.RUnlock()
	}
	return doc
}

func importKey(u, dest string) string { return u + "\x00" + dest }

// ImportDownloads adds the exported items as paused jobs without any network
// probe (the list may be restored while offline). Items already present with the
// same URL and destination, and finished items, are skipped.
func (m *JobManager) ImportDownloads(doc ExportDoc) ImportResult {
	var res ImportResult

	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]struct{}, len(m.jobs))
	for _, j := range m.jobs {
		seen[importKey(j.GetURL(), j.GetDestPath())] = struct{}{}
	}

	for _, it := range doc.Downloads {
		u := strings.TrimSpace(it.URL)
		if ValidateURL(u) != nil {
			res.Invalid++
			continue
		}
		dest, err := ResolveDestPath(m.opt.Out, it.DestPath)
		if err != nil {
			res.Invalid++
			continue
		}
		if it.Status == StatusCompleted {
			res.Skipped++
			continue
		}
		k := importKey(u, dest)
		if _, dup := seen[k]; dup {
			res.Skipped++
			continue
		}
		seen[k] = struct{}{}

		name := strings.TrimSpace(it.FileName)
		if name == "" {
			if p, perr := url.Parse(u); perr == nil {
				name = path.Base(p.Path)
			}
			if name == "" || name == "/" || name == "." {
				name = "download"
			}
		}
		sum, algo := it.Checksum, strings.ToLower(it.ChecksumAlgo)
		if sum != "" && algo != "sha256" && algo != "md5" {
			sum, algo = "", "" // unusable checksum: import the link without it
		}
		job := &Job{
			ID: uuid.New().String(), URL: u, FileName: name, Status: StatusPaused,
			CreatedAt: time.Now(), DestPath: dest, Category: it.Category,
			Checksum: sum, ChecksumAlgo: algo,
			SpeedLimit: it.SpeedLimit, MaxRetries: it.MaxRetries,
		}
		m.jobs[job.ID] = job
		if _, taken := m.urls[u]; !taken {
			m.urls[u] = job.ID
		}
		res.Added++
	}
	if res.Added > 0 {
		m.saveToDiskLocked()
	}
	return res
}
