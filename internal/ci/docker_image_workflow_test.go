package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerImageWorkflowBuildsMultiArchitectureExtensionImage(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/docker-image.yml")

	requiredSnippets := []string{
		"uses: docker/setup-qemu-action@v3",
		"uses: docker/setup-buildx-action@v3",
		"platforms: linux/amd64,linux/arm64",
		"images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}",
		"type=sha,prefix=sha-",
		"push: ${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(workflow, snippet) {
			t.Fatalf("docker image workflow missing %q", snippet)
		}
	}
}

func TestDockerfileCrossCompilesForTargetPlatform(t *testing.T) {
	dockerfile := readRepositoryFile(t, "Dockerfile")

	requiredSnippets := []string{
		"FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build",
		"ARG TARGETOS",
		"ARG TARGETARCH",
		"GOOS=${TARGETOS} GOARCH=${TARGETARCH}",
		`HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD ["/miwkey-extension", "healthcheck"]`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(dockerfile, snippet) {
			t.Fatalf("Dockerfile missing %q", snippet)
		}
	}
}

func readRepositoryFile(t *testing.T, name string) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	for {
		path := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(path); err == nil {
			content, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			return string(content)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repository root not found from %s", dir)
		}
		dir = parent
	}
}
