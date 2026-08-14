package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// NovelJobStatus mirrors a novel job's status snapshot from the server.
type NovelJobStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"` // running | done | error
	Stage  string `json:"stage"`
	Kind   string `json:"kind"` // generate | adjust
	Error  string `json:"error"`
}

// NovelText returns the session's saved novelization, a version tag for
// optimistic-concurrency saves, and whether one exists yet.
func (c *Client) NovelText(ctx context.Context, name string) (text, version string, exists bool, err error) {
	var out struct {
		Text    string `json:"text"`
		Version string `json:"version"`
		Exists  bool   `json:"exists"`
	}
	err = c.do(ctx, "GET", "/api/sessions/"+enc(name)+"/novel", nil, &out)
	return out.Text, out.Version, out.Exists, err
}

// SaveNovelText saves an edited novelization with optimistic concurrency
// (baseVersion must match the stored version) and returns the new version.
func (c *Client) SaveNovelText(ctx context.Context, name, text, baseVersion string) (string, error) {
	var out struct {
		Version string `json:"version"`
	}
	err := c.do(ctx, "PUT", "/api/sessions/"+enc(name)+"/novel",
		map[string]string{"text": text, "base_version": baseVersion}, &out)
	return out.Version, err
}

// StartNovelJob asks the server to (re)generate the session's novel; the result
// is persisted server-side. Returns the job to poll with NovelJob.
func (c *Client) StartNovelJob(ctx context.Context, name string) (NovelJobStatus, error) {
	var out NovelJobStatus
	err := c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/novel", nil, &out)
	return out, err
}

// StartNovelAdjustJob asks the server to revise the novel with the AI. If
// selection is non-empty only that excerpt is revised; the result is fetched
// with NovelJobResult and is NOT persisted server-side.
func (c *Client) StartNovelAdjustJob(ctx context.Context, name, text, selection, instruction string) (NovelJobStatus, error) {
	var out NovelJobStatus
	err := c.do(ctx, "POST", "/api/sessions/"+enc(name)+"/novel/adjust",
		map[string]string{"text": text, "selection": selection, "instruction": instruction}, &out)
	return out, err
}

// NovelJob polls a novel job's status.
func (c *Client) NovelJob(ctx context.Context, id string) (NovelJobStatus, error) {
	var out NovelJobStatus
	err := c.do(ctx, "GET", "/api/novel-jobs/"+enc(id), nil, &out)
	return out, err
}

// NovelJobResult returns a finished job's text (e.g. an adjustment's revised
// prose).
func (c *Client) NovelJobResult(ctx context.Context, id string) (string, error) {
	var out struct {
		Text string `json:"text"`
		Kind string `json:"kind"`
	}
	err := c.do(ctx, "GET", "/api/novel-jobs/"+enc(id)+"/result", nil, &out)
	return out.Text, err
}

// DownloadSessionNovel fetches the session's SAVED novel as raw bytes, in
// format "md" (default) or "pdf".
func (c *Client) DownloadSessionNovel(ctx context.Context, name, format string) ([]byte, error) {
	path := "/api/sessions/" + enc(name) + "/novel/download"
	if format == "pdf" {
		path += "?format=pdf"
	}
	return c.getBytes(ctx, path)
}

// getBytes performs an authenticated GET and returns the raw response body — for
// endpoints that stream a file (md/pdf) rather than JSON.
func (c *Client) getBytes(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.base+path, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return nil, fmt.Errorf("%s (HTTP %d)", e.Error, resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
