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
  minSpecVersion: "v2.8"
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

const managedServiceBundle = `
apiVersion: obacht.dev/v2
kind: Template
metadata:
  name: camera-streamer
  displayName: Camera Streamer
  version: "1.0.0"
spec:
  minSpecVersion: "v2.8"
  minSudoLevel: power
  compatibility:
    devices: [raspberry-pi-4, raspberry-pi-5]
    architectures: [linux/arm64]
    requiresFeatures: [csi-or-usb-camera]
  runtime:
    type: system
    system:
      managed_service:
        kind: mediamtx
        binary: mediamtx
        binary_url: https://github.com/bluenviron/mediamtx/releases/download/v1.0.0/mediamtx.tar.gz
        binary_digest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
        archive: tgz
        args: ["/etc/obacht/svc/${instance.id}/mediamtx.yml"]
        hardware:
          groups: [video]
          devices: ["/dev/video*", "/dev/media*"]
        listen_ports: [8888]
      files:
        - path: /etc/obacht/svc/${instance.id}/mediamtx.yml
          content: "hls: yes"
  configSchema:
    - key: camera
      label: Kamera
      type: select
      required: true
      optionsSource:
        kind: device_inventory
        inventory: cameras
`

func TestUnmarshalManagedService(t *testing.T) {
	var m Manifest
	if err := yaml.Unmarshal([]byte(managedServiceBundle), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if m.Spec.Runtime.Type != "system" || m.Spec.Runtime.System == nil {
		t.Fatalf("expected system runtime, got %+v", m.Spec.Runtime)
	}
	ms := m.Spec.Runtime.System.ManagedService
	if ms == nil {
		t.Fatal("managed_service: nil")
	}
	if ms.Binary != "mediamtx" || ms.Archive != "tgz" {
		t.Errorf("managed_service basics: %+v", ms)
	}
	if ms.Hardware == nil || len(ms.Hardware.Groups) != 1 || ms.Hardware.Groups[0] != "video" {
		t.Errorf("hardware.groups: %+v", ms.Hardware)
	}
	if len(ms.Hardware.Devices) != 2 || ms.Hardware.Devices[0] != "/dev/video*" {
		t.Errorf("hardware.devices: %+v", ms.Hardware.Devices)
	}
	if len(ms.ListenPorts) != 1 || ms.ListenPorts[0] != 8888 {
		t.Errorf("listen_ports: %+v", ms.ListenPorts)
	}
	if m.Spec.Runtime.System.UnitName != "" || m.Spec.Runtime.System.UnitTemplate != "" {
		t.Errorf("withdrawn flavor fields must stay empty, got %+v", m.Spec.Runtime.System)
	}
	if len(m.Spec.Compatibility.RequiresFeatures) != 1 || m.Spec.Compatibility.RequiresFeatures[0] != "csi-or-usb-camera" {
		t.Errorf("requiresFeatures: %+v", m.Spec.Compatibility.RequiresFeatures)
	}
	if len(m.Spec.ConfigSchema) != 1 {
		t.Fatalf("configSchema: %+v", m.Spec.ConfigSchema)
	}
	os := m.Spec.ConfigSchema[0].OptionsSource
	if os == nil || os.Kind != "device_inventory" || os.Inventory != "cameras" {
		t.Errorf("optionsSource: %+v", os)
	}
}

const kioskBundle = `
apiVersion: obacht.dev/v2
kind: Template
metadata:
  name: kiosk-mode
  displayName: Kiosk
  version: "1.0.0"
spec:
  minSpecVersion: "v2.8"
  minSudoLevel: power
  exclusivityGroup: display-output
  compatibility:
    devices: [raspberry-pi-4, raspberry-pi-5]
    architectures: [linux/arm64]
    requiresFeatures: [desktop-chromium, wayland-compositor]
  runtime:
    type: system
    system:
      kiosk: {}
      files:
        - path: /etc/obacht/kiosk/config.env
          content: "KIOSK_URL=${cfg.url}"
`

func TestUnmarshalKiosk(t *testing.T) {
	var m Manifest
	if err := yaml.Unmarshal([]byte(kioskBundle), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if m.Spec.Runtime.System == nil || m.Spec.Runtime.System.Kiosk == nil {
		t.Fatalf("expected kiosk flavor, got %+v", m.Spec.Runtime)
	}
	if m.Spec.ExclusivityGroup != "display-output" {
		t.Errorf("exclusivityGroup: %q", m.Spec.ExclusivityGroup)
	}
	if len(m.Spec.Runtime.System.Files) != 1 {
		t.Errorf("files: %+v", m.Spec.Runtime.System.Files)
	}
}
