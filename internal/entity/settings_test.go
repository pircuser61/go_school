package entity

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func TestExternalSystem_ValidateInputMapping(t *testing.T) {
	t.Parallel()

	t.Run("Valid with filled required", func(t *testing.T) {
		t.Parallel()

		es := &ExternalSystem{
			InputMapping: &script.JSONSchema{
				Type: "object",
				Properties: script.JSONSchemaProperties{
					"a": {
						Type:  "string",
						Title: "Тестовая строка",
						Value: "test-mapping",
					},
				},
				Required: []string{"a"},
			},
		}

		err := es.ValidateInputMapping()

		assert.NoError(t, err)
	})

	t.Run("Valid with required and default", func(t *testing.T) {
		t.Parallel()

		es := &ExternalSystem{
			InputMapping: &script.JSONSchema{
				Type: "object",
				Properties: script.JSONSchemaProperties{
					"a": {
						Type:    "string",
						Title:   "Тестовая строка",
						Default: "Тестовые данные",
					},
				},
				Required: []string{"a"},
			},
		}

		err := es.ValidateInputMapping()

		assert.NoError(t, err)
	})

	t.Run("Valid with required and nested object", func(t *testing.T) {
		t.Parallel()

		es := &ExternalSystem{
			InputMapping: &script.JSONSchema{
				Type: "object",
				Properties: script.JSONSchemaProperties{
					"a-obj": {
						Type: "object",
						Properties: script.JSONSchemaProperties{
							"a-str": {
								Type:  "string",
								Title: "Тестовая строка в объекте",
								Value: "test-mapping",
							},
						},
						Required: []string{"a-str"},
					},
				},
			},
		}

		err := es.ValidateInputMapping()

		assert.NoError(t, err)
	})

	t.Run("Invalid with required and nested object", func(t *testing.T) {
		t.Parallel()

		es := &ExternalSystem{
			InputMapping: &script.JSONSchema{
				Type: "object",
				Properties: script.JSONSchemaProperties{
					"a-obj": {
						Type: "object",
						Properties: script.JSONSchemaProperties{
							"a-str": {
								Type:  "string",
								Title: "Тестовая строка в объекте",
							},
						},
						Required: []string{"a-str"},
					},
				},
			},
		}

		err := es.ValidateInputMapping()

		assert.EqualError(t, err, fmt.Sprintf("%s: %s", ErrMappingRequired.Error(), "a-obj.a-str"))
	})

	t.Run("Invalid with required", func(t *testing.T) {
		t.Parallel()

		es := &ExternalSystem{
			InputMapping: &script.JSONSchema{
				Type: "object",
				Properties: script.JSONSchemaProperties{
					"a": {
						Type:  "string",
						Title: "Тестовая строка",
					},
				},
				Required: []string{"a"},
			},
		}

		err := es.ValidateInputMapping()

		assert.EqualError(t, err, fmt.Sprintf("%s: %s", ErrMappingRequired.Error(), "a"))
	})

	t.Run("Empty mapping", func(t *testing.T) {
		t.Parallel()

		es := &ExternalSystem{
			InputSchema: &script.JSONSchema{
				Type: "object",
				Properties: script.JSONSchemaProperties{
					"a": {
						Type:  "string",
						Title: "Тестовая строка",
					},
				},
			},
		}

		err := es.ValidateInputMapping()

		assert.NoError(t, err)
	})
}
