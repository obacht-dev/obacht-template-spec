// Package manifest defines Go types for the obacht.dev/v2 template manifest,
// spec revision v2.9.
//
// The schema is owned by https://github.com/obacht-dev/obacht-template-spec —
// keep these types in sync with schema/manifest-v2.json and ts/manifest-v2.ts.
// The agent uses these types to deserialise manifests fetched from the
// registry; for full structural validation, validate against the JSON Schema
// in schema/manifest-v2.json instead of writing a hand-rolled validator. The
// small Validate helpers on Readiness and PostStartHook exist because the
// agent must enforce those contracts at apply time regardless of what the
// registry checked.
package manifest

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	APIVersionV2 = "obacht.dev/v2"
	// SupportedSpecVersion is the highest additive spec revision modelled here:
	// v2.2 immutable config/secret fields; v2.3 tag-only compose images +
	// envConfigKey; v2.4 macOS platform (mac device, darwin/arm64, excludeDevices,
	// system host_service); v2.5 spec.gettingStarted; v2.6/v2.7 render-only
	// config-field additions (typed renderers, advanced flag — no Go type
	// changes); v2.8 the Pi system-runtime flavors managed_service + kiosk,
	// compatibility.requiresFeatures and configField.optionsSource. v2.8 also
	// withdraws the never-shipped free-form systemd flavor (unitName +
	// unitTemplate) — decoders keep the fields only to detect and reject it.
	// v2.9 configField.optionRequirements, the render-only spec.hosting hints,
	// services[].readiness (post-install HTTP readiness probe), the macOS
	// host_service.post_start hooks and the ${host.data} placeholder.
	SupportedSpecVersion = "v2.9"
)

// Host placeholders are resolved by the agent at apply time from facts only
// the device knows. They survive api-side materialisation (see the agent's
// BuildInstanceConfig), so a validator that rejects unresolved placeholders
// must let every name in HostPlaceholders through.
const (
	// HostPlaceholderGateway (v2.4) is the VZ gateway IP a VM container uses
	// to reach the macOS host, e.g. "http://${host.gateway}:11434".
	HostPlaceholderGateway = "${host.gateway}"
	// HostPlaceholderData (v2.9) is the instance's managed data directory for
	// a host service, created by the agent before the service starts (on
	// macOS ~/Library/Application Support/obacht/host/data/<launchd label>).
	// Allowed in host_service.env values and host_service.args.
	HostPlaceholderData = "${host.data}"
)

// HostPlaceholders lists every ${host.*} placeholder the agent resolves.
var HostPlaceholders = []string{HostPlaceholderGateway, HostPlaceholderData}

// IsHostPlaceholder reports whether name (with or without the ${...} wrapper,
// e.g. "host.data" or "${host.data}") is a known host placeholder.
func IsHostPlaceholder(name string) bool {
	if !strings.HasPrefix(name, "${") {
		name = "${" + name + "}"
	}
	for _, p := range HostPlaceholders {
		if p == name {
			return true
		}
	}
	return false
}

// Manifest is the top-level v2 template manifest.
type Manifest struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind"       json:"kind"`
	Metadata   Metadata `yaml:"metadata"   json:"metadata"`
	Spec       Spec     `yaml:"spec"       json:"spec"`
}

// Metadata holds the descriptive fields shown in the dashboard.
type Metadata struct {
	Name        string   `yaml:"name"                  json:"name"`
	DisplayName string   `yaml:"displayName"           json:"displayName"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string   `yaml:"version"               json:"version"`
	Author      string   `yaml:"author,omitempty"      json:"author,omitempty"`
	License     string   `yaml:"license,omitempty"     json:"license,omitempty"`
	Homepage    string   `yaml:"homepage,omitempty"    json:"homepage,omitempty"`
	Icon        string   `yaml:"icon,omitempty"        json:"icon,omitempty"`
	Tags        []string `yaml:"tags,omitempty"        json:"tags,omitempty"`
	TrustLevel  string   `yaml:"trustLevel,omitempty"  json:"trustLevel,omitempty"`
}

// Spec is the actionable part of the manifest — what to run and how to bind it.
type Spec struct {
	// MinSpecVersion names the spec revision the manifest depends on
	// (e.g. "v2.1"). Agents refuse manifests requesting a higher
	// revision than they support.
	MinSpecVersion   string         `yaml:"minSpecVersion"             json:"minSpecVersion"`
	MinAgentVersion  string         `yaml:"minAgentVersion,omitempty"  json:"minAgentVersion,omitempty"`
	ExclusivityGroup string         `yaml:"exclusivityGroup,omitempty" json:"exclusivityGroup,omitempty"`
	Compatibility    Compatibility  `yaml:"compatibility"              json:"compatibility"`
	Runtime          Runtime        `yaml:"runtime"                    json:"runtime"`
	Services         []Service      `yaml:"services,omitempty"         json:"services,omitempty"`
	ConfigSchema     []ConfigField  `yaml:"configSchema,omitempty"     json:"configSchema,omitempty"`
	SecretsSchema    []SecretField  `yaml:"secretsSchema,omitempty"    json:"secretsSchema,omitempty"`
	Provides         []ProvideEntry `yaml:"provides,omitempty"         json:"provides,omitempty"`
	Consumes         []ConsumeEntry `yaml:"consumes,omitempty"         json:"consumes,omitempty"`

	// MinSudoLevel is the minimum host privilege the template needs.
	// "" / "none" = docker-only sandbox; "power" = needs Power Mode unlocked.
	MinSudoLevel string `yaml:"minSudoLevel,omitempty" json:"minSudoLevel,omitempty"`

	// Secrets lists env-var keys whose values must be redacted from
	// telemetry, audit logs, and propagated error messages.
	Secrets []string `yaml:"secrets,omitempty" json:"secrets,omitempty"`

	// GettingStarted (v2.5) is an optional post-install note shown to the user
	// (e.g. "open the app and register your account"). Informational only.
	GettingStarted string `yaml:"gettingStarted,omitempty" json:"gettingStarted,omitempty"`

	// Hosting (v2.9, render-only) carries hints for the Hosting step. Basic
	// Auth is a property of the attached domain, never of the template; the
	// agent and api ignore this block.
	Hosting *Hosting `yaml:"hosting,omitempty" json:"hosting,omitempty"`
}

// Hosting holds the v2.9 render-only Hosting-step hints.
type Hosting struct {
	// RecommendBasicAuth asks clients that attach a domain to an instance of
	// this template to pre-enable per-domain Basic Auth.
	RecommendBasicAuth bool `yaml:"recommendBasicAuth,omitempty" json:"recommendBasicAuth,omitempty"`
	// BasicAuthReason is the short user-facing reason shown next to the toggle.
	BasicAuthReason string `yaml:"basicAuthReason,omitempty" json:"basicAuthReason,omitempty"`
}

// Compatibility is the pre-flight contract validated by the agent before
// any image is pulled.
type Compatibility struct {
	Devices []string `yaml:"devices,omitempty" json:"devices,omitempty"`
	// ExcludeDevices (v2.4) names device classes the template must NOT be
	// offered on even if the architecture matches (clients filter on this).
	ExcludeDevices []string `yaml:"excludeDevices,omitempty" json:"excludeDevices,omitempty"`
	Architectures  []string `yaml:"architectures"            json:"architectures"`
	// RequiresFeatures (v2.8) names device features the template requires
	// (e.g. "desktop-chromium", "wayland-compositor", "csi-or-usb-camera").
	// The agent detects and reports features; api compat check, client filter
	// and the on-device install assertion all enforce the subset relation.
	RequiresFeatures []string        `yaml:"requiresFeatures,omitempty" json:"requiresFeatures,omitempty"`
	OS               []OSRequirement `yaml:"os,omitempty"               json:"os,omitempty"`
	Resources        *ResourceBudget `yaml:"resources,omitempty"        json:"resources,omitempty"`
}

// OSRequirement describes one acceptable host OS family.
type OSRequirement struct {
	ID         string `yaml:"id"                   json:"id"`
	MinVersion string `yaml:"minVersion,omitempty" json:"minVersion,omitempty"`
}

// ResourceBudget declares minimum host resources required.
type ResourceBudget struct {
	MinRamMb  int64 `yaml:"minRamMb,omitempty"  json:"minRamMb,omitempty"`
	MinDiskMb int64 `yaml:"minDiskMb,omitempty" json:"minDiskMb,omitempty"`
}

// Runtime is a discriminated union over Type. Exactly one of
// Container/Compose/System must be populated; check Type before reading them.
type Runtime struct {
	Type      string     `yaml:"type"                json:"type"`
	Container *Container `yaml:"container,omitempty" json:"container,omitempty"`
	Compose   *Compose   `yaml:"compose,omitempty"   json:"compose,omitempty"`
	System    *System    `yaml:"system,omitempty"    json:"system,omitempty"`
}

// Container is the inputs needed to run one Docker container.
type Container struct {
	Image       string            `yaml:"image"                 json:"image"`
	ImageDigest string            `yaml:"imageDigest,omitempty" json:"imageDigest,omitempty"`
	Cmd         []string          `yaml:"cmd,omitempty"         json:"cmd,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"         json:"env,omitempty"`
	Ports       []PortMap         `yaml:"ports,omitempty"       json:"ports,omitempty"`
	Volumes     []VolumeMount     `yaml:"volumes,omitempty"     json:"volumes,omitempty"`
	Network     string            `yaml:"network,omitempty"     json:"network,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"      json:"labels,omitempty"`
}

// Compose is the multi-container bundle runtime.
//
// Body is the raw YAML compose document. The agent parses Body, applies
// ${cfg.x} / ${secret.x} / ${instance.id} substitutions, walks the
// allowlist (defence-in-depth — the registry should have already done
// this), rewrites every image: line to image: ref@digest from
// ImageDigests, and hands the result to docker compose.
type Compose struct {
	PrimaryService string            `yaml:"primaryService"           json:"primaryService"`
	PrimaryPort    int               `yaml:"primaryPort"              json:"primaryPort"`
	DataPath       string            `yaml:"dataPath,omitempty"       json:"dataPath,omitempty"`
	ImageDigests   map[string]string `yaml:"imageDigests,omitempty"   json:"imageDigests,omitempty"`
	Body           string            `yaml:"body"                     json:"body"`
	// AllowUnpinnedImages relaxes digest pinning for user-supplied compose
	// bodies (custom-docker-composition). EnvConfigKey names the config key
	// whose value is written as the project .env file.
	AllowUnpinnedImages bool   `yaml:"allowUnpinnedImages,omitempty" json:"allowUnpinnedImages,omitempty"`
	EnvConfigKey        string `yaml:"envConfigKey,omitempty"        json:"envConfigKey,omitempty"`
}

// PortMap is a TCP host→container port pair. Host=0 means assign at install time.
type PortMap struct {
	Host      int `yaml:"host"      json:"host"`
	Container int `yaml:"container" json:"container"`
}

// VolumeMount mounts a named volume into the container.
type VolumeMount struct {
	Source   string `yaml:"source"             json:"source"`
	Target   string `yaml:"target"             json:"target"`
	ReadOnly bool   `yaml:"readOnly,omitempty" json:"readOnly,omitempty"`
}

// System describes a host-managed workload. Exactly one flavor is present:
// the v2.4 macOS HostService (launchd-managed host binary), the v2.8 Linux
// ManagedService (agent-generated hardened systemd unit), or the v2.8 Kiosk
// marker (agent-shipped kiosk session).
type System struct {
	// UnitName/UnitTemplate are the WITHDRAWN pre-v2.8 free-form systemd
	// flavor. No template ever shipped it. The fields are kept only so
	// decoders can detect a manifest using it and reject it explicitly —
	// system templates never author unit text.
	UnitName     string       `yaml:"unitName,omitempty"     json:"unitName,omitempty"`
	UnitTemplate string       `yaml:"unitTemplate,omitempty" json:"unitTemplate,omitempty"`
	Files        []SystemFile `yaml:"files,omitempty"        json:"files,omitempty"`
	// HostService (v2.4) is the macOS launchd flavor: a structured host binary
	// (binary + argv + env) instead of a systemd unit. The agent verifies
	// BinaryDigest before extract/exec.
	HostService *HostService `yaml:"host_service,omitempty" json:"host_service,omitempty"`
	// ManagedService (v2.8) is the Raspberry Pi / Linux flavor: a digest-pinned
	// host binary run as a hardened systemd unit the agent generates
	// (DynamicUser, DevicePolicy=closed + declared DeviceAllow,
	// NoNewPrivileges, ProtectSystem=strict). Requires minSudoLevel: power.
	ManagedService *ManagedService `yaml:"managed_service,omitempty" json:"managed_service,omitempty"`
	// Kiosk (v2.8) marks the agent-shipped kiosk session flavor. Always an
	// empty object — all privileged behaviour lives in the agent; the template
	// contributes configSchema + files only. Requires minSudoLevel: power.
	Kiosk *KioskFlavor `yaml:"kiosk,omitempty" json:"kiosk,omitempty"`
}

// KioskFlavor is the empty v2.8 kiosk marker.
type KioskFlavor struct{}

// ManagedService is the v2.8 Linux flavor of the system runtime.
type ManagedService struct {
	Kind         string            `yaml:"kind,omitempty"    json:"kind,omitempty"`
	Binary       string            `yaml:"binary"            json:"binary"`
	BinaryURL    string            `yaml:"binary_url"        json:"binary_url"`
	BinaryDigest string            `yaml:"binary_digest"     json:"binary_digest"`
	Archive      string            `yaml:"archive,omitempty" json:"archive,omitempty"`
	Args         []string          `yaml:"args,omitempty"    json:"args,omitempty"`
	Env          map[string]string `yaml:"env,omitempty"     json:"env,omitempty"`
	// Hardware declares the access the generated unit grants the workload.
	// Closed enums (groups: video/render/input; devices: /dev/video*,
	// /dev/media*, /dev/dri/*, /dev/dma_heap/*) — mapped to systemd DeviceAllow.
	Hardware *ManagedServiceHardware `yaml:"hardware,omitempty" json:"hardware,omitempty"`
	// ListenPorts documents the local ports the service binds (validation
	// only; exposure happens exclusively via spec.services → device Caddy).
	ListenPorts []int `yaml:"listen_ports,omitempty" json:"listen_ports,omitempty"`
}

// ManagedServiceHardware is the hardware-access grant block of ManagedService.
type ManagedServiceHardware struct {
	Groups  []string `yaml:"groups,omitempty"  json:"groups,omitempty"`
	Devices []string `yaml:"devices,omitempty" json:"devices,omitempty"`
}

// HostService is the v2.4 macOS host-service flavor of the system runtime.
//
// Args and Env values may contain ${cfg.X}, ${host.gateway} and (v2.9)
// ${host.data}; the agent resolves the ${host.*} ones at apply time, see
// HostPlaceholders.
type HostService struct {
	Kind         string            `yaml:"kind,omitempty"      json:"kind,omitempty"`
	Binary       string            `yaml:"binary"              json:"binary"`
	BinaryURL    string            `yaml:"binary_url"          json:"binary_url"`
	BinaryDigest string            `yaml:"binary_digest"       json:"binary_digest"`
	Archive      string            `yaml:"archive,omitempty"   json:"archive,omitempty"`
	Args         []string          `yaml:"args,omitempty"      json:"args,omitempty"`
	Env          map[string]string `yaml:"env,omitempty"       json:"env,omitempty"`
	DataDir      string            `yaml:"data_dir,omitempty"  json:"data_dir,omitempty"`
	// PostStart (v2.9) lists HTTP hooks the agent runs after launchd has
	// loaded the service. The agent waits (retrying up to 120 s) until the
	// first hook's URL accepts a TCP connection, then runs the hooks in order;
	// a hook succeeds on a 2xx response. service_ready is "starting" while
	// hooks run, "ready" when all succeeded, "unhealthy" if one fails after
	// the agent's retries. Hooks run on every (re)start of the service, so
	// they must be idempotent.
	PostStart []PostStartHook `yaml:"post_start,omitempty" json:"post_start,omitempty"`
}

// PostStartHook is one entry of HostService.PostStart. Exactly one key is
// allowed today: HTTP.
type PostStartHook struct {
	HTTP *PostStartHTTP `yaml:"http,omitempty" json:"http,omitempty"`
}

// PostStartHTTP is a loopback HTTP request the agent sends after the host
// service is up. URL and Body may contain ${cfg.X} placeholders.
type PostStartHTTP struct {
	// Method is "GET" or "POST".
	Method string `yaml:"method" json:"method"`
	// URL must match PostStartURLPattern (loopback only); the agent refuses
	// anything else.
	URL  string `yaml:"url"            json:"url"`
	Body string `yaml:"body,omitempty" json:"body,omitempty"`
	// ContentType defaults to DefaultPostStartContentType.
	ContentType string `yaml:"contentType,omitempty" json:"contentType,omitempty"`
	// TimeoutSeconds is the per-hook response timeout; 0 means
	// DefaultPostStartTimeoutSeconds. Allowed range 10..7200.
	TimeoutSeconds int `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`
}

const (
	DefaultPostStartContentType    = "application/json"
	DefaultPostStartTimeoutSeconds = 1800
	MinPostStartTimeoutSeconds     = 10
	MaxPostStartTimeoutSeconds     = 7200
	// PostStartURLPattern is the loopback-only URL shape post_start hooks
	// must use. Kept as a string so other validators can embed it.
	PostStartURLPattern = `^http://(127\.0\.0\.1|localhost)(:[0-9]+)?/`
)

var postStartURLRe = regexp.MustCompile(PostStartURLPattern)

// Validate checks the structural contract of one post_start hook: exactly the
// http key, a GET or POST method, a loopback URL and a timeout inside the
// allowed range. Placeholders in URL and Body are left alone.
func (h PostStartHook) Validate() error {
	if h.HTTP == nil {
		return fmt.Errorf("post_start hook: http is required")
	}
	r := h.HTTP
	if r.Method != "GET" && r.Method != "POST" {
		return fmt.Errorf("post_start hook: method must be GET or POST, got %q", r.Method)
	}
	if !postStartURLRe.MatchString(r.URL) {
		return fmt.Errorf("post_start hook: url must be a loopback http URL (http://127.0.0.1/... or http://localhost/...), got %q", r.URL)
	}
	if r.TimeoutSeconds != 0 && (r.TimeoutSeconds < MinPostStartTimeoutSeconds || r.TimeoutSeconds > MaxPostStartTimeoutSeconds) {
		return fmt.Errorf("post_start hook: timeoutSeconds must be %d..%d, got %d", MinPostStartTimeoutSeconds, MaxPostStartTimeoutSeconds, r.TimeoutSeconds)
	}
	return nil
}

// EffectiveContentType returns ContentType or the default.
func (r *PostStartHTTP) EffectiveContentType() string {
	if r == nil || r.ContentType == "" {
		return DefaultPostStartContentType
	}
	return r.ContentType
}

// EffectiveTimeoutSeconds returns TimeoutSeconds or the default.
func (r *PostStartHTTP) EffectiveTimeoutSeconds() int {
	if r == nil || r.TimeoutSeconds == 0 {
		return DefaultPostStartTimeoutSeconds
	}
	return r.TimeoutSeconds
}

// SystemFile is a sidecar file written to disk before the unit starts.
type SystemFile struct {
	Path    string `yaml:"path"           json:"path"`
	Mode    string `yaml:"mode,omitempty" json:"mode,omitempty"`
	Content string `yaml:"content"        json:"content"`
}

// Service is an ingress-bindable port/socket the template exposes.
type Service struct {
	Name string `yaml:"name" json:"name"`
	// TargetType is "container_port" | "host_port" | "unix_socket".
	TargetType string `yaml:"targetType" json:"targetType"`
	// TargetService is the compose service name for ingress to route to.
	// Required when runtime.type=compose.
	TargetService string `yaml:"targetService,omitempty" json:"targetService,omitempty"`
	TargetPort    int    `yaml:"targetPort,omitempty"    json:"targetPort,omitempty"`
	TargetPath    string `yaml:"targetPath,omitempty"    json:"targetPath,omitempty"`
	// AppPath (v2.8): when this service is bound at the domain root, the agent
	// redirects a bare "/" here (apps that serve under a subpath, e.g.
	// MediaMTX at "/cam/"). Absolute path; additive — old agents ignore it.
	AppPath string `yaml:"appPath,omitempty" json:"appPath,omitempty"`
	// Readiness (v2.9) tunes the HTTP readiness probe the agent runs after it
	// reports the instance as installed. Nil means the defaults (see
	// Readiness): the agent still probes "/" for container and compose
	// runtimes; the block only tunes it.
	Readiness *Readiness `yaml:"readiness,omitempty" json:"readiness,omitempty"`
}

// Readiness describes the post-install HTTP probe of one service. The agent
// sends GET requests to Path (through the device's Caddy when a domain is
// bound, otherwise directly) and reports service_ready as "starting" until
// the first accepted status, then "ready"; after TimeoutSeconds without
// success "unhealthy". Clients hold the install wizard's ready screen until
// the app really answers.
type Readiness struct {
	// Path defaults to "/". Must start with "/", must not start with "//"
	// and must not contain "://".
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
	// ExpectStatus lists the HTTP status codes that count as ready. Empty
	// means the default: any 200..399 plus 401 and 403 (an auth wall still
	// means the app answers). Each value must be 100..599.
	ExpectStatus []int `yaml:"expectStatus,omitempty" json:"expectStatus,omitempty"`
	// TimeoutSeconds is how long the agent keeps probing before it reports
	// "unhealthy"; 0 means DefaultReadinessTimeoutSeconds. Allowed 10..1800.
	TimeoutSeconds int `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`
}

const (
	DefaultReadinessPath           = "/"
	DefaultReadinessTimeoutSeconds = 300
	MinReadinessTimeoutSeconds     = 10
	MaxReadinessTimeoutSeconds     = 1800
)

// Validate checks the structural contract of a readiness block. A nil
// receiver is valid (defaults apply).
func (r *Readiness) Validate() error {
	if r == nil {
		return nil
	}
	if r.Path != "" {
		if !strings.HasPrefix(r.Path, "/") || strings.HasPrefix(r.Path, "//") || strings.Contains(r.Path, "://") {
			return fmt.Errorf("readiness.path must be an absolute path without scheme or host, got %q", r.Path)
		}
	}
	for i, s := range r.ExpectStatus {
		if s < 100 || s > 599 {
			return fmt.Errorf("readiness.expectStatus[%d] must be 100..599, got %d", i, s)
		}
	}
	if r.TimeoutSeconds != 0 && (r.TimeoutSeconds < MinReadinessTimeoutSeconds || r.TimeoutSeconds > MaxReadinessTimeoutSeconds) {
		return fmt.Errorf("readiness.timeoutSeconds must be %d..%d, got %d", MinReadinessTimeoutSeconds, MaxReadinessTimeoutSeconds, r.TimeoutSeconds)
	}
	return nil
}

// EffectivePath returns Path or "/". Safe on a nil receiver.
func (r *Readiness) EffectivePath() string {
	if r == nil || r.Path == "" {
		return DefaultReadinessPath
	}
	return r.Path
}

// EffectiveTimeoutSeconds returns TimeoutSeconds or the default. Safe on a
// nil receiver.
func (r *Readiness) EffectiveTimeoutSeconds() int {
	if r == nil || r.TimeoutSeconds == 0 {
		return DefaultReadinessTimeoutSeconds
	}
	return r.TimeoutSeconds
}

// AcceptsStatus reports whether an HTTP status code counts as ready: the
// listed ExpectStatus codes, or with none listed any 200..399 plus 401 and
// 403. Safe on a nil receiver.
func (r *Readiness) AcceptsStatus(code int) bool {
	if r == nil || len(r.ExpectStatus) == 0 {
		return (code >= 200 && code <= 399) || code == 401 || code == 403
	}
	for _, s := range r.ExpectStatus {
		if s == code {
			return true
		}
	}
	return false
}

// ConfigField is one input rendered as a form field at install time.
type ConfigField struct {
	Key         string         `yaml:"key"                   json:"key"`
	Label       string         `yaml:"label"                 json:"label"`
	Type        string         `yaml:"type"                  json:"type"`
	Required    bool           `yaml:"required,omitempty"    json:"required,omitempty"`
	Default     interface{}    `yaml:"default,omitempty"     json:"default,omitempty"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Options     []ConfigOption `yaml:"options,omitempty"     json:"options,omitempty"`
	// OptionsSource (v2.8, render-only, mutually exclusive with Options):
	// select options come from device-reported inventory (e.g. detected
	// cameras). The agent and api treat the chosen value as a plain string.
	OptionsSource *OptionsSource `yaml:"optionsSource,omitempty" json:"optionsSource,omitempty"`
	// OptionRequirements (v2.9, select fields with static Options only) maps
	// an option value to the minimum resources that option needs on top of
	// Compatibility.Resources. The agent's disk preflight uses the chosen
	// option's MinDiskMb; clients grey out options the device can't satisfy.
	OptionRequirements map[string]ResourceBudget `yaml:"optionRequirements,omitempty" json:"optionRequirements,omitempty"`

	// For type=service_reference (declared but not resolved in v2.1).
	Interface        string          `yaml:"interface,omitempty"        json:"interface,omitempty"`
	InterfaceVersion string          `yaml:"interfaceVersion,omitempty" json:"interfaceVersion,omitempty"`
	Fallback         *ConfigFallback `yaml:"fallback,omitempty"         json:"fallback,omitempty"`

	// Immutable (v2.2): value is set at install time and cannot be changed
	// afterwards (first-boot bootstrap values).
	Immutable bool `yaml:"immutable,omitempty" json:"immutable,omitempty"`
}

// ConfigOption is one entry of a select-type ConfigField.
type ConfigOption struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label" json:"label"`
}

// OptionsSource points a select field at device-reported inventory (v2.8).
// Kind is always "device_inventory"; Inventory names the inventory list
// ("cameras" in v2.8).
type OptionsSource struct {
	Kind      string `yaml:"kind"      json:"kind"`
	Inventory string `yaml:"inventory" json:"inventory"`
}

// ConfigFallback is the input shown when no provider exists for a service_reference.
type ConfigFallback struct {
	Type        string `yaml:"type"                  json:"type"`
	Placeholder string `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Default     string `yaml:"default,omitempty"     json:"default,omitempty"`
}

// SecretField declares a per-instance secret the agent generates with
// crypto/rand. Reference it in the runtime body via ${secret.<key>}.
type SecretField struct {
	Key     string `yaml:"key"               json:"key"`
	Length  int    `yaml:"length"            json:"length"`
	Charset string `yaml:"charset,omitempty" json:"charset,omitempty"`
	// Immutable (v2.2): secrets are immutable by default once generated; the
	// flag is accepted for explicit declaration.
	Immutable bool `yaml:"immutable,omitempty" json:"immutable,omitempty"`
}

// ProvideEntry declares one service interface the template advertises.
// Resolution and cross-bundle network wiring is reserved for phase 2;
// in v2.1 the agent ignores this block.
type ProvideEntry struct {
	Interface string `yaml:"interface" json:"interface"`
	Version   string `yaml:"version"   json:"version"`
	Service   string `yaml:"service"   json:"service"`
	Port      int    `yaml:"port"      json:"port"`
	Path      string `yaml:"path,omitempty" json:"path,omitempty"`
	Auth      string `yaml:"auth,omitempty" json:"auth,omitempty"`
}

// ConsumeEntry declares one service interface the template consumes.
// In v2.1 the configKey it points at is rendered as a normal text input
// in the webapp; phase 2 will turn it into a provider picker.
type ConsumeEntry struct {
	Interface string `yaml:"interface" json:"interface"`
	Version   string `yaml:"version"   json:"version"`
	ConfigKey string `yaml:"configKey" json:"configKey"`
}
