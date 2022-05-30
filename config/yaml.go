package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

func parseYaml(out interface{}, blob []byte) error {
	dec := yaml.NewDecoder(bytes.NewReader(blob))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("can't parse yaml: %w", err)
	}
	return nil
}
