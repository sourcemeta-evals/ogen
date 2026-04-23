package parser

import (
	"fmt"

	"github.com/go-faster/errors"

	"github.com/ogen-go/ogen/openapi"
)

func (p *parser) parseVersion() (rerr error) {
	defer func() {
		rerr = p.wrapLocation(p.rootFile, p.rootLoc.Field("openapi"), rerr)
	}()

	version := p.spec.OpenAPI
	if version == "" {
		version = p.spec.Swagger
	}

	if err := p.version.UnmarshalText([]byte(version)); err != nil {
		return errors.Wrap(err, "invalid version")
	}
	// Allow OpenAPI 3.2 specs.
	// The query and additionalOperations fields require version 3.2.
	// See https://spec.openapis.org/oas/v3.2.0#path-item-object.
	// Version gating is enforced in parsePathItem.
	// We only need to accept the version here.
	// The actual validation happens downstream.
	if p.version.Major != 3 || p.version.Minor > 2 {
		return errors.Errorf("unsupported version: %s", version)
	}
	return nil
}

// FeatureVersionError is an error that is returned when a feature is used
// that requires a newer version of OpenAPI.
type FeatureVersionError struct {
	Feature string
	Minimum openapi.Version
	Actual  openapi.Version
}

// Error implements error.
func (f *FeatureVersionError) Error() string {
	return fmt.Sprintf("feature %q requires OpenAPI version %s, but actual version is %s",
		f.Feature, f.Minimum, f.Actual,
	)
}

func (p *parser) requireMinorVersion(feature string, minor int) error {
	if p.version.Minor >= minor {
		return nil
	}
	return &FeatureVersionError{
		Feature: feature,
		Minimum: openapi.Version{
			Major: p.version.Major,
			Minor: minor,
		},
		Actual: p.version,
	}
}
