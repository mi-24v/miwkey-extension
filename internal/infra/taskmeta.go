package infra

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	envTaskMetadataURI = "ECS_CONTAINER_METADATA_URI_V4"

	taskMetadataTimeout     = 2 * time.Second
	taskMetadataMaxBodySize = 1 << 20

	// UnknownTaskARN is recorded when the task metadata endpoint is not
	// configured or does not answer in time.
	UnknownTaskARN = "unknown"
)

// FetchTaskARN reads the task ARN from the ECS task metadata endpoint (v4)
// within taskMetadataTimeout. It never consults other sources such as IMDS
// and returns UnknownTaskARN on any failure.
func FetchTaskARN(ctx context.Context, client *http.Client, getenv func(string) string) string {
	base := getenv(envTaskMetadataURI)
	if base == "" {
		return UnknownTaskARN
	}

	ctx, cancel := context.WithTimeout(ctx, taskMetadataTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/task", nil)
	if err != nil {
		return UnknownTaskARN
	}

	// Use a copy so the caller's client (possibly http.DefaultClient) keeps its
	// redirect policy; a redirect must never lead to another source such as IMDS.
	noRedirect := *client
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	res, err := noRedirect.Do(req)
	if err != nil {
		return UnknownTaskARN
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return UnknownTaskARN
	}

	var task struct {
		TaskARN string `json:"TaskARN"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, taskMetadataMaxBodySize)).Decode(&task); err != nil || task.TaskARN == "" {
		return UnknownTaskARN
	}

	return task.TaskARN
}
