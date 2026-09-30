package infra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const testTaskARN = "arn:aws:ecs:ap-northeast-1:123456789012:task/c/abc"

func metadataEnv(uri string) func(string) string {
	return envMap(map[string]string{"ECS_CONTAINER_METADATA_URI_V4": uri})
}

func TestFetchTaskARNReadsTaskEndpoint(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Cluster":"c","TaskARN":"` + testTaskARN + `","Family":"f"}`))
	}))
	defer server.Close()

	got := FetchTaskARN(context.Background(), server.Client(), metadataEnv(server.URL+"/v4/container-id"))

	assert.Equal(t, testTaskARN, got)
	assert.Equal(t, []string{"GET /v4/container-id/task"}, paths)
}

func TestFetchTaskARNWithoutURIReturnsUnknown(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	got := FetchTaskARN(context.Background(), server.Client(), envMap(map[string]string{}))

	assert.Equal(t, "unknown", got)
	assert.Zero(t, calls.Load())
}

func TestFetchTaskARNTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
			_, _ = w.Write([]byte(`{"TaskARN":"` + testTaskARN + `"}`))
		}
	}))
	defer server.Close()

	start := time.Now()
	got := FetchTaskARN(context.Background(), server.Client(), metadataEnv(server.URL))
	elapsed := time.Since(start)

	assert.Equal(t, "unknown", got)
	assert.Less(t, elapsed, 2500*time.Millisecond)
	assert.GreaterOrEqual(t, elapsed, 1500*time.Millisecond)
}

func TestFetchTaskARNNon2xxOrBadJSONReturnsUnknown(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"500": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"TaskARN":"` + testTaskARN + `"}`))
		},
		"404": func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		},
		"redirect": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		},
		"bad JSON": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"TaskARN":`))
		},
		"missing TaskARN": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Cluster":"c"}`))
		},
		"TaskARN wrong type": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"TaskARN":42}`))
		},
	}

	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()

			assert.Equal(t, "unknown", FetchTaskARN(context.Background(), server.Client(), metadataEnv(server.URL)))
		})
	}
}

func TestFetchTaskARNInvalidURIReturnsUnknown(t *testing.T) {
	assert.Equal(t, "unknown", FetchTaskARN(context.Background(), http.DefaultClient, metadataEnv("://not a url")))
}

func TestFetchTaskARNDoesNotFollowRedirects(t *testing.T) {
	var otherHits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherHits.Add(1)
		_, _ = w.Write([]byte(`{"TaskARN":"arn:aws:ecs:ap-northeast-1:123456789012:task/c/redirected"}`))
	}))
	defer other.Close()

	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/latest/meta-data/", http.StatusFound)
	}))
	defer metadata.Close()

	provided := metadata.Client()
	for name, client := range map[string]*http.Client{
		"provided client":    provided,
		"http.DefaultClient": http.DefaultClient,
	} {
		t.Run(name, func(t *testing.T) {
			otherHits.Store(0)

			got := FetchTaskARN(context.Background(), client, metadataEnv(metadata.URL))

			assert.Equal(t, "unknown", got)
			assert.Zero(t, otherHits.Load(), "the redirect target must never be contacted")
			assert.Nil(t, client.CheckRedirect, "the caller's client must not be mutated")
		})
	}
}
