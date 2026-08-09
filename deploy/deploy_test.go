package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeploymentArtifactsExist(t *testing.T) {
	root := ".."

	t.Run("Dockerfile exists and contains multi-stage Go build", func(t *testing.T) {
		dockerfilePath := filepath.Join(root, "Dockerfile")
		content, err := os.ReadFile(dockerfilePath)
		if err != nil {
			t.Fatalf("failed to read Dockerfile: %v", err)
		}

		str := string(content)
		if !strings.Contains(str, "FROM golang:") || !strings.Contains(str, "FROM gcr.io/distroless") {
			t.Errorf("Dockerfile should be a multi-stage build starting from golang and distroless")
		}
	})

	t.Run("docker-compose.yml exists and defines microservices", func(t *testing.T) {
		composePath := filepath.Join(root, "docker-compose.yml")
		content, err := os.ReadFile(composePath)
		if err != nil {
			t.Fatalf("failed to read docker-compose.yml: %v", err)
		}

		str := string(content)
		services := []string{"lostpet-service", "foundpet-service", "pet-matcher", "notification-service", "ollama"}
		for _, svc := range services {
			if !strings.Contains(str, svc) {
				t.Errorf("docker-compose.yml missing service %s", svc)
			}
		}
	})

	t.Run("docker-compose.yml bootstraps the Ollama model before pet-matcher", func(t *testing.T) {
		composePath := filepath.Join(root, "docker-compose.yml")
		content, err := os.ReadFile(composePath)
		if err != nil {
			t.Fatalf("failed to read docker-compose.yml: %v", err)
		}

		str := string(content)
		required := []string{
			"healthcheck:",
			"ollama-init:",
			"condition: service_healthy",
			"condition: service_completed_successfully",
			"command: [\"pull\", \"${OLLAMA_MODEL:-gemma4:e2b}\"]",
		}
		for _, fragment := range required {
			if !strings.Contains(str, fragment) {
				t.Errorf("docker-compose.yml missing Ollama bootstrap configuration %q", fragment)
			}
		}
		if strings.Contains(str, `"11434:11434"`) {
			t.Error("docker-compose.yml should not publish Ollama's port by default")
		}
	})

	t.Run("Cloud Run service manifest YAMLs exist", func(t *testing.T) {
		manifests := []string{
			"lostpet-service.yaml",
			"foundpet-service.yaml",
			"pet-matcher.yaml",
			"notification-service.yaml",
		}

		for _, m := range manifests {
			path := filepath.Join(root, "deploy", "cloudrun", m)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("missing Cloud Run manifest %s: %v", m, err)
				continue
			}

			str := string(content)
			if !strings.Contains(str, "apiVersion: serving.knative.dev/v1") || !strings.Contains(str, "kind: Service") {
				t.Errorf("manifest %s missing Knative Service API header", m)
			}
		}
	})
}
