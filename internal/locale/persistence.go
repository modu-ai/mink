package locale

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// localeConfigSection mirrors the `locale:` section in ~/.mink/config.yaml.
// Used for YAML round-trip serialization (AC-LC-015, REQ-LC-011).
//
// The path is ~/.mink/config.yaml under the `locale:` key, consistent with
// SPEC-MINK-ONBOARDING-001 §6.0 global config policy.
type localeConfigSection struct {
	// Override is an optional user-supplied LocaleContext that wins over OS/IP detection.
	Override *LocaleContext `yaml:"override,omitempty"`

	// GeolocationEnabled controls whether IP geolocation fallback is attempted.
	// Default true (when key is absent from YAML, defaults to false in Go zero-value,
	// but Detect() uses true when not explicitly configured).
	GeolocationEnabled *bool `yaml:"geolocation_enabled,omitempty"`

	// GeoIPDBPath is an optional path to a MaxMind GeoLite2-Country.mmdb file.
	GeoIPDBPath string `yaml:"geoip_db_path,omitempty"`
}

// yamlDocument is a minimal envelope used to parse only the `locale:` section
// from a broader config.yaml that may contain other unrelated keys.
type yamlDocument struct {
	Locale localeConfigSection `yaml:"locale"`
}

// Load reads a LocaleContext from the `locale.override` sub-key in the YAML
// provided by reader. It returns the override LocaleContext when present, or a
// zero-value LocaleContext (with empty Country) when no override is configured.
//
// Unknown YAML fields inside the `locale:` block are silently ignored to support
// forward compatibility (consuming SPECs may add keys LOCALE-001 does not know).
func Load(reader io.Reader) (LocaleContext, error) {
	var doc yamlDocument
	dec := yaml.NewDecoder(reader)
	dec.KnownFields(false) // tolerate unknown fields for forward compat
	if err := dec.Decode(&doc); err != nil && err != io.EOF {
		return LocaleContext{}, fmt.Errorf("locale: decode yaml: %w", err)
	}

	if doc.Locale.Override == nil {
		return LocaleContext{}, nil
	}
	return *doc.Locale.Override, nil
}

// Save serialises lc into the `locale.override` YAML sub-key and writes it to
// writer. The output is a valid YAML document fragment that can be embedded into
// ~/.mink/config.yaml under the `locale:` key.
//
// Callers are responsible for merging this output with the rest of config.yaml;
// this function only writes the `locale:` section.
func Save(writer io.Writer, lc LocaleContext) error {
	doc := yamlDocument{
		Locale: localeConfigSection{
			Override: &lc,
		},
	}

	enc := yaml.NewEncoder(writer)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("locale: encode yaml: %w", err)
	}
	return enc.Close()
}

// LoadConfig reads the full LocaleConfig section (override + geolocation settings)
// from the YAML provided by reader.
func LoadConfig(reader io.Reader) (localeConfigSection, error) {
	var doc yamlDocument
	dec := yaml.NewDecoder(reader)
	dec.KnownFields(false)
	if err := dec.Decode(&doc); err != nil && err != io.EOF {
		return localeConfigSection{}, fmt.Errorf("locale: decode config yaml: %w", err)
	}
	return doc.Locale, nil
}
