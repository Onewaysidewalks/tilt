//go:build !skipcontainertests && !windows
// +build !skipcontainertests,!windows

// Tests that involve spinning up/interacting with actual containers
package build

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// * * * IMAGE BUILDER * * *

func TestDockerBuildDockerfile(t *testing.T) {
	f := newDockerBuildFixture(t)

	df := dockerfile.Dockerfile(`
FROM alpine
WORKDIR /src
ADD a.txt .
RUN cp a.txt b.txt
ADD dir/c.txt .
`)

	f.WriteFile("a.txt", "a")
	f.WriteFile("dir/c.txt", "c")
	f.WriteFile("missing.txt", "missing")

	spec := v1alpha1.DockerImageSpec{
		DockerfileContents: df.String(),
		Context:            f.Path(),
	}
	refs, _, err := f.b.BuildImage(f.ctx, f.ps, f.getNameFromTest(), spec,
		defaultCluster,
		nil,
		model.EmptyMatcher)
	if err != nil {
		t.Fatal(err)
	}

	f.assertImageHasLabels(refs.LocalRef, docker.BuiltByTiltLabel)

	pcs := []expectedFile{
		expectedFile{Path: "/src/a.txt", Contents: "a"},
		expectedFile{Path: "/src/b.txt", Contents: "a"},
		expectedFile{Path: "/src/c.txt", Contents: "c"},
		expectedFile{Path: "/src/dir/c.txt", Missing: true},
		expectedFile{Path: "/src/missing.txt", Missing: true},
	}
	f.assertFilesInImage(refs.LocalRef, pcs)
}

func TestDockerBuildWithBuildArgs(t *testing.T) {
	f := newDockerBuildFixture(t)

	df := dockerfile.Dockerfile(`FROM alpine
ARG some_variable_name

ADD $some_variable_name /test.txt`)

	f.WriteFile("awesome_variable", "hi im an awesome variable")

	spec := v1alpha1.DockerImageSpec{
		DockerfileContents: df.String(),
		Context:            f.Path(),
		Args:               []string{"some_variable_name=awesome_variable"},
	}
	refs, _, err := f.b.BuildImage(f.ctx, f.ps, f.getNameFromTest(), spec,
		defaultCluster,
		nil,
		model.EmptyMatcher)
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "/test.txt", Contents: "hi im an awesome variable"},
	}
	f.assertFilesInImage(refs.LocalRef, expected)
}

func TestDockerBuildWithExtraTags(t *testing.T) {
	f := newDockerBuildFixture(t)

	df := dockerfile.Dockerfile(`
FROM alpine
WORKDIR /src
ADD a.txt .`)

	f.WriteFile("a.txt", "a")

	spec := v1alpha1.DockerImageSpec{
		DockerfileContents: df.String(),
		Context:            f.Path(),
		ExtraTags:          []string{"fe:jenkins-1234"},
		Args:               []string{"some_variable_name=awesome_variable"},
	}
	refs, _, err := f.b.BuildImage(f.ctx, f.ps, f.getNameFromTest(), spec,
		defaultCluster,
		nil,
		model.EmptyMatcher)
	if err != nil {
		t.Fatal(err)
	}

	f.assertImageHasLabels(refs.LocalRef, docker.BuiltByTiltLabel)

	pcs := []expectedFile{
		expectedFile{Path: "/src/a.txt", Contents: "a"},
	}
	f.assertFilesInImage(container.MustParseNamedTagged("fe:jenkins-1234"), pcs)
}

func TestDetectBuildkitCorruption(t *testing.T) {
	f := newDockerBuildFixture(t)

	out := bytes.NewBuffer(nil)
	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(out))
	ps := NewPipelineState(ctx, 1, ProvideClock())

	spec := v1alpha1.DockerImageSpec{
		// Simulate buildkit corruption
		DockerfileContents: `FROM alpine
RUN echo 'failed to create LLB definition: failed commit on ref "unknown-sha256:b72fa303a3a5fbf52c723bfcfb93948bb53b3d7e8d22418e9d171a27ad7dcd84": "unknown-sha256:b72fa303a3a5fbf52c723bfcfb93948bb53b3d7e8d22418e9d171a27ad7dcd84" failed size validation: 80941 != 80929: failed precondition' && exit 1
`,
		Context: f.Path(),
	}
	_, _, err := f.b.BuildImage(ctx, ps, f.getNameFromTest(), spec,
		defaultCluster,
		nil,
		model.EmptyMatcher)
	assert.Error(t, err)
	assert.Contains(t, out.String(), "Detected Buildkit corruption. Rebuilding without Buildkit")
	assert.Contains(t, out.String(), "[1/2] FROM docker.io/library/alpine") // buildkit-style output
	assert.Contains(t, out.String(), "Step 1/3 : FROM alpine")              // Legacy output
}
