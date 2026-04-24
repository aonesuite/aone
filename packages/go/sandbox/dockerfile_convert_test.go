package sandbox

import (
	"strings"
	"testing"
)

func TestFromDockerfile(t *testing.T) {
	content := strings.Join([]string{
		"FROM ubuntu:22.04",
		"RUN apt-get update && apt-get install -y curl",
		"WORKDIR /app",
		"ENV FOO=bar",
		"COPY . /app",
		"CMD [\"sleep\", \"infinity\"]",
	}, "\n")

	b, err := FromDockerfile(content)
	if err != nil {
		t.Fatalf("FromDockerfile: %v", err)
	}
	if b == nil {
		t.Fatal("nil builder")
	}
}

func TestConvertDockerfile(t *testing.T) {
	content := "FROM alpine:3.19\nRUN echo hello\n"
	result, err := ConvertDockerfile(content)
	if err != nil {
		t.Fatalf("ConvertDockerfile: %v", err)
	}
	if result.BaseImage != "alpine:3.19" {
		t.Fatalf("base image %q", result.BaseImage)
	}
	// Expect: USER root, WORKDIR /, RUN echo hello, USER user, WORKDIR /home/user
	if len(result.Steps) < 5 {
		t.Fatalf("expected at least 5 steps, got %d", len(result.Steps))
	}
}

func TestConvertDockerfileRequiresFROM(t *testing.T) {
	_, err := ConvertDockerfile("RUN echo hi\n")
	if err == nil {
		t.Fatal("expected error for missing FROM")
	}
}
