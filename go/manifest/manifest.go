// Package manifest defines Go types for the obacht.dev/v2 template manifest,
// spec revision v2.1.
//
// The schema is owned by https://github.com/obacht-dev/obacht-template-spec —
// keep these types in sync with schema/manifest-v2.json and ts/manifest-v2.ts.
// The agent uses these types to deserialise manifests fetched from the
// registry; for full structural validation, validate against the JSON Schema
// in schema/manifest-v2.json instead of writing a hand-rolled validator.
package manifest

const (
	APIVersionV2         = "obacht.dev/v2"
	SupportedSpecVersion = "v2.1"
)

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
}

// Compatibility is the pre-flight contract validated by the agent before
// any image is pulled.
type Compatibility struct {
	Devices       []string         `yaml:"devices,omitempty"       json:"devices,omitempty"`
	Architectures []string         `yaml:"architectures"           json:"architectures"`
	OS            []OSRequirement  `yaml:"os,omitempty"            json:"os,omitempty"`
	Resources     *ResourceBudget  `yaml:"resources,omitempty"     json:"resources,omitempty"`
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

// System describes a systemd-managed workload.
type System struct {
	UnitName     string       `yaml:"unitName"        json:"unitName"`
	UnitTemplate string       `yaml:"unitTemplate"    json:"unitTemplate"`
	Files        []SystemFile `yaml:"files,omitempty" json:"files,omitempty"`
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

	// For type=service_reference (declared but not resolved in v2.1).
	Interface        string             `yaml:"interface,omitempty"        json:"interface,omitempty"`
	InterfaceVersion string             `yaml:"interfaceVersion,omitempty" json:"interfaceVersion,omitempty"`
	Fallback         *ConfigFallback    `yaml:"fallback,omitempty"         json:"fallback,omitempty"`
}

// ConfigOption is one entry of a select-type ConfigField.
type ConfigOption struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label" json:"label"`
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
