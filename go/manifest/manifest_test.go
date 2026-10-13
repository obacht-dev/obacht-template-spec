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
  minSpecVersion: "v2.9"
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

const ollamaHost = `
apiVersion: obacht.dev/v2
kind: Template
metadata:
  name: ollama-host
  displayName: Ollama (Mac)
  version: "1.2.0"
spec:
  minSpecVersion: "v2.9"
  compatibility:
    devices: [mac]
    architectures: [darwin/arm64]
  runtime:
    type: system
    system:
      host_service:
        kind: ollama
        binary: ollama
        binary_url: https://github.com/ollama/ollama/releases/download/v0.40.0/ollama-darwin.tgz
        binary_digest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
        archive: tgz
        args: [serve]
        env:
          OLLAMA_HOST: "0.0.0.0:11434"
          OLLAMA_MODELS: "${host.data}/models"
        post_start:
          - http:
              method: POST
              url: http://127.0.0.1:11434/api/pull
              body: '{"name": "${cfg.model}", "stream": false}'
              timeoutSeconds: 3600
          - http:
              method: GET
              url: http://localhost:11434/api/tags
  services:
    - name: api
      targetType: host_port
      targetPort: 11434
      readiness:
        path: /api/version
        expectStatus: [200]
        timeoutSeconds: 120
  configSchema:
    - key: model
      label: Model
      type: text
      default: gemma3:4b
`

func TestUnmarshalHostServicePostStart(t *testing.T) {
	var m Manifest
	if err := yaml.Unmarshal([]byte(ollamaHost), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	hs := m.Spec.Runtime.System.HostService
	if hs == nil {
		t.Fatal("host_service: nil")
	}
	if hs.Env["OLLAMA_MODELS"] != "${host.data}/models" {
		t.Errorf("env ${host.data} must survive decoding, got %q", hs.Env["OLLAMA_MODELS"])
	}
	if len(hs.PostStart) != 2 {
		t.Fatalf("post_start: want 2 hooks, got %+v", hs.PostStart)
	}
	first := hs.PostStart[0]
	if first.HTTP == nil || first.HTTP.Method != "POST" || first.HTTP.URL != "http://127.0.0.1:11434/api/pull" {
		t.Errorf("post_start[0]: %+v", first.HTTP)
	}
	if first.HTTP.Body != `{"name": "${cfg.model}", "stream": false}` {
		t.Errorf("post_start[0].body: %q", first.HTTP.Body)
	}
	if first.HTTP.EffectiveTimeoutSeconds() != 3600 || first.HTTP.EffectiveContentType() != DefaultPostStartContentType {
		t.Errorf("post_start[0] effective values: timeout=%d contentType=%q", first.HTTP.EffectiveTimeoutSeconds(), first.HTTP.EffectiveContentType())
	}
	second := hs.PostStart[1]
	if second.HTTP == nil || second.HTTP.Method != "GET" || second.HTTP.EffectiveTimeoutSeconds() != DefaultPostStartTimeoutSeconds {
		t.Errorf("post_start[1]: %+v", second.HTTP)
	}
	for i, h := range hs.PostStart {
		if err := h.Validate(); err != nil {
			t.Errorf("post_start[%d].Validate: %v", i, err)
		}
	}
	if len(m.Spec.Services) != 1 {
		t.Fatalf("services: %+v", m.Spec.Services)
	}
	rd := m.Spec.Services[0].Readiness
	if rd == nil || rd.Path != "/api/version" || rd.TimeoutSeconds != 120 || len(rd.ExpectStatus) != 1 || rd.ExpectStatus[0] != 200 {
		t.Errorf("readiness: %+v", rd)
	}
	if err := rd.Validate(); err != nil {
		t.Errorf("readiness.Validate: %v", err)
	}
	if !rd.AcceptsStatus(200) || rd.AcceptsStatus(302) || rd.AcceptsStatus(401) {
		t.Errorf("explicit expectStatus must be exact: %+v", rd.ExpectStatus)
	}
}

func TestReadinessDefaults(t *testing.T) {
	var r *Readiness
	if err := r.Validate(); err != nil {
		t.Errorf("nil readiness must be valid: %v", err)
	}
	if r.EffectivePath() != "/" || r.EffectiveTimeoutSeconds() != 300 {
		t.Errorf("nil defaults: path=%q timeout=%d", r.EffectivePath(), r.EffectiveTimeoutSeconds())
	}
	for _, code := range []int{200, 204, 301, 399, 401, 403} {
		if !r.AcceptsStatus(code) {
			t.Errorf("default must accept %d", code)
		}
	}
	for _, code := range []int{100, 199, 400, 404, 500, 502, 503} {
		if r.AcceptsStatus(code) {
			t.Errorf("default must reject %d", code)
		}
	}
	empty := &Readiness{}
	if empty.EffectivePath() != "/" || empty.EffectiveTimeoutSeconds() != 300 || !empty.AcceptsStatus(302) {
		t.Errorf("empty block must behave like nil: %+v", empty)
	}
}

func TestReadinessValidate(t *testing.T) {
	bad := []Readiness{
		{Path: "health"},
		{Path: "//evil.example"},
		{Path: "/x?u=http://evil.example"},
		{ExpectStatus: []int{99}},
		{ExpectStatus: []int{200, 600}},
		{TimeoutSeconds: 9},
		{TimeoutSeconds: 1801},
	}
	for i, r := range bad {
		r := r
		if err := r.Validate(); err == nil {
			t.Errorf("bad[%d] %+v: expected error", i, r)
		}
	}
	good := []Readiness{
		{},
		{Path: "/"},
		{Path: "/health"},
		{Path: "/api/v1/status?probe=1"},
		{ExpectStatus: []int{200, 401}},
		{TimeoutSeconds: 10},
		{TimeoutSeconds: 1800},
	}
	for i, r := range good {
		r := r
		if err := r.Validate(); err != nil {
			t.Errorf("good[%d] %+v: %v", i, r, err)
		}
	}
}

func TestPostStartHookValidate(t *testing.T) {
	bad := []PostStartHook{
		{},
		{HTTP: &PostStartHTTP{Method: "PUT", URL: "http://127.0.0.1:11434/api/pull"}},
		{HTTP: &PostStartHTTP{Method: "GET", URL: "https://127.0.0.1:11434/"}},
		{HTTP: &PostStartHTTP{Method: "GET", URL: "http://10.0.0.1:11434/"}},
		{HTTP: &PostStartHTTP{Method: "GET", URL: "http://127.0.0.1.evil.example/"}},
		{HTTP: &PostStartHTTP{Method: "GET", URL: "http://localhost:11434"}},
		{HTTP: &PostStartHTTP{Method: "GET", URL: "http://127.0.0.1:11434/", TimeoutSeconds: 9}},
		{HTTP: &PostStartHTTP{Method: "GET", URL: "http://127.0.0.1:11434/", TimeoutSeconds: 7201}},
	}
	for i, h := range bad {
		if err := h.Validate(); err == nil {
			t.Errorf("bad[%d] %+v: expected error", i, h.HTTP)
		}
	}
	good := []PostStartHook{
		{HTTP: &PostStartHTTP{Method: "GET", URL: "http://127.0.0.1:11434/api/tags"}},
		{HTTP: &PostStartHTTP{Method: "POST", URL: "http://localhost/api/pull", Body: "{}"}},
		{HTTP: &PostStartHTTP{Method: "POST", URL: "http://127.0.0.1:11434/api/${cfg.action}", TimeoutSeconds: 7200}},
	}
	for i, h := range good {
		if err := h.Validate(); err != nil {
			t.Errorf("good[%d] %+v: %v", i, h.HTTP, err)
		}
	}
}

func TestIsHostPlaceholder(t *testing.T) {
	for _, name := range []string{"host.gateway", "${host.gateway}", "host.data", "${host.data}"} {
		if !IsHostPlaceholder(name) {
			t.Errorf("%q must be a host placeholder", name)
		}
	}
	for _, name := range []string{"host.other", "${cfg.model}", "secret.x", ""} {
		if IsHostPlaceholder(name) {
			t.Errorf("%q must not be a host placeholder", name)
		}
	}
}
