// Package manifest defines Go types for the obacht.dev/v2 template manifest.
//
// The schema is owned by https://github.com/obacht-dev/obacht-template-spec —
// keep these types in sync with schema/manifest-v2.json and ts/manifest-v2.ts.
// The agent uses these types to deserialise manifests fetched from the
// registry; for full structural validation, validate against the JSON Schema
// in schema/manifest-v2.json instead of writing a hand-rolled validator.
package manifest

const APIVersionV2 = "obacht.dev/v2"

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
	MinAgentVersion  string         `yaml:"minAgentVersion,omitempty"  json:"minAgentVersion,omitempty"`
	ExclusivityGroup string         `yaml:"exclusivityGroup,omitempty" json:"exclusivityGroup,omitempty"`
	Runtime          Runtime        `yaml:"runtime"                    json:"runtime"`
	Services         []Service      `yaml:"services,omitempty"         json:"services,omitempty"`
	ConfigSchema     []ConfigField  `yaml:"configSchema,omitempty"     json:"configSchema,omitempty"`
}

// Runtime is a discriminated union over Type. Exactly one of Container/System
// must be populated; check Type before reading them.
type Runtime struct {
	Type      string     `yaml:"type"                json:"type"`
	Container *Container `yaml:"container,omitempty" json:"container,omitempty"`
	System    *System    `yaml:"system,omitempty"    json:"system,omitempty"`
}

// Container is the inputs needed to run one Docker container.
type Container struct {
	Image   string            `yaml:"image"             json:"image"`
	Cmd     []string          `yaml:"cmd,omitempty"     json:"cmd,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"     json:"env,omitempty"`
	Ports   []PortMap         `yaml:"ports,omitempty"   json:"ports,omitempty"`
	Volumes []VolumeMount     `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Network string            `yaml:"network,omitempty" json:"network,omitempty"`
	Labels  map[string]string `yaml:"labels,omitempty"  json:"labels,omitempty"`
}

// PortMap is a TCP host→container port pair. Host=0 means assign at install time.
type PortMap struct {
	Host      int `yaml:"host"      json:"host"`
	Container int `yaml:"container" json:"container"`
}

// VolumeMount mounts a host path or named volume into the container.
type VolumeMount struct {
	Source   string `yaml:"source"             json:"source"`
	Target   string `yaml:"target"             json:"target"`
	ReadOnly bool   `yaml:"readOnly,omitempty" json:"readOnly,omitempty"`
}

// System describes a systemd-managed workload. Used when the workload cannot
// run inside Docker (e.g. needs the host display server).
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

// Service is an ingress-bindable port/socket the template exposes. The
// device's Caddy maps a domain to one of these.
type Service struct {
	Name       string `yaml:"name"                 json:"name"`
	TargetType string `yaml:"targetType"           json:"targetType"`
	TargetPort int    `yaml:"targetPort,omitempty" json:"targetPort,omitempty"`
	TargetPath string `yaml:"targetPath,omitempty" json:"targetPath,omitempty"`
}

// ConfigField is one input rendered as a form field at install time. The
// user's value is interpolated into the runtime spec via ${cfg.<Key>}.
type ConfigField struct {
	Key         string         `yaml:"key"                   json:"key"`
	Label       string         `yaml:"label"                 json:"label"`
	Type        string         `yaml:"type"                  json:"type"`
	Required    bool           `yaml:"required,omitempty"    json:"required,omitempty"`
	Default     interface{}    `yaml:"default,omitempty"     json:"default,omitempty"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Options     []ConfigOption `yaml:"options,omitempty"     json:"options,omitempty"`
}

// ConfigOption is one entry of a select-type ConfigField.
type ConfigOption struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label" json:"label"`
}
