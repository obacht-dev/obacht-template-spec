package manifest

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const ghostBundle = `
apiVersion: obacht.dev/v2
kind: Template
metadata:
  name: ghost-bundle
  displayName: Ghost
  version: "1.0.0"
spec:
  minSpecVersion: "v2.5"
  minAgentVersion: "0.3.0"
  compatibility:
    architectures: [linux/arm64]
    resources:
      minRamMb: 768
      minDiskMb: 2048
  runtime:
    type: compose
    compose:
      primaryService: web
      primaryPort: 2368
      body: |
        services:
          web:
            image: ghost:5-alpine
  services:
    - name: web
      targetType: container_port
      targetService: web
      targetPort: 2368
  secretsSchema:
    - key: db_root_password
      length: 32
`

func TestUnmarshalComposeBundle(t *testing.T) {
	var m Manifest
	if err := yaml.Unmarshal([]byte(ghostBundle), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if m.APIVersion != APIVersionV2 {
		t.Errorf("apiVersion: got %q want %q", m.APIVersion, APIVersionV2)
	}
	if m.Spec.MinSpecVersion != SupportedSpecVersion {
		t.Errorf("minSpecVersion: got %q want %q", m.Spec.MinSpecVersion, SupportedSpecVersion)
	}
	if m.Spec.Runtime.Type != "compose" {
		t.Fatalf("runtime.type: got %q want compose", m.Spec.Runtime.Type)
	}
	if m.Spec.Runtime.Compose == nil {
		t.Fatal("runtime.compose: nil")
	}
	if m.Spec.Runtime.Compose.PrimaryService != "web" {
		t.Errorf("primaryService: got %q want web", m.Spec.Runtime.Compose.PrimaryService)
	}
	if !strings.Contains(m.Spec.Runtime.Compose.Body, "image: ghost:5-alpine") {
		t.Errorf("body did not roundtrip cleanly")
	}
	if len(m.Spec.SecretsSchema) != 1 || m.Spec.SecretsSchema[0].Key != "db_root_password" {
		t.Errorf("secretsSchema: got %+v", m.Spec.SecretsSchema)
	}
	if len(m.Spec.Services) != 1 || m.Spec.Services[0].TargetService != "web" {
		t.Errorf("services[0].targetService: got %+v", m.Spec.Services)
	}
	if len(m.Spec.Compatibility.Architectures) != 1 || m.Spec.Compatibility.Architectures[0] != "linux/arm64" {
		t.Errorf("compatibility.architectures: got %+v", m.Spec.Compatibility.Architectures)
	}
}

const containerSimple = `
apiVersion: obacht.dev/v2
kind: Template
metadata:
  name: whoami
  displayName: whoami
  version: "1.0.0"
spec:
  minSpecVersion: "v2.5"
  minAgentVersion: "0.3.0"
  compatibility:
    architectures: [linux/arm64, linux/amd64]
  runtime:
    type: container
    container:
      image: traefik/whoami:latest
  services:
    - name: web
      targetType: container_port
      targetPort: 80
`

func TestUnmarshalContainer(t *testing.T) {
	var m Manifest
	if err := yaml.Unmarshal([]byte(containerSimple), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if m.Spec.Runtime.Type != "container" || m.Spec.Runtime.Container == nil {
		t.Fatalf("expected container runtime, got %+v", m.Spec.Runtime)
	}
	if m.Spec.Runtime.Container.Image != "traefik/whoami:latest" {
		t.Errorf("image: %q", m.Spec.Runtime.Container.Image)
	}
}
