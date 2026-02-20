package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"environment", "Environment"},
		{"project_deployment", "ProjectDeployment"},
		{"container_registry", "ContainerRegistry"},
		{"api_key", "ApiKey"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertAttribute(t *testing.T) {
	tests := []struct {
		name     string
		input    AttributeSpec
		expected AttributeData
	}{
		{
			name: "required string attribute",
			input: AttributeSpec{
				Name:        "name",
				Description: "The name",
				String: &TypeSpec{
					ComputedOptionalRequired: "required",
					Description:              "A name field",
				},
			},
			expected: AttributeData{
				Name:        "name",
				FieldName:   "Name",
				TFName:      "name",
				Type:        "String",
				SchemaType:  "String",
				Description: "The name",
				Required:    true,
				Optional:    false,
				Computed:    false,
				Sensitive:   false,
			},
		},
		{
			name: "optional bool attribute",
			input: AttributeSpec{
				Name: "enabled",
				Bool: &TypeSpec{
					ComputedOptionalRequired: "optional",
					Description:              "Enable feature",
				},
			},
			expected: AttributeData{
				Name:        "enabled",
				FieldName:   "Enabled",
				TFName:      "enabled",
				Type:        "Bool",
				SchemaType:  "Bool",
				Description: "Enable feature",
				Required:    false,
				Optional:    true,
				Computed:    false,
				Sensitive:   false,
			},
		},
		{
			name: "computed int64 attribute",
			input: AttributeSpec{
				Name: "count",
				Int64: &TypeSpec{
					ComputedOptionalRequired: "computed",
				},
			},
			expected: AttributeData{
				Name:        "count",
				FieldName:   "Count",
				TFName:      "count",
				Type:        "Int64",
				SchemaType:  "Int64",
				Description: "",
				Required:    false,
				Optional:    false,
				Computed:    true,
				Sensitive:   false,
			},
		},
		{
			name: "sensitive string attribute",
			input: AttributeSpec{
				Name: "api_key",
				String: &TypeSpec{
					ComputedOptionalRequired: "computed",
					Sensitive:                true,
				},
			},
			expected: AttributeData{
				Name:        "api_key",
				FieldName:   "ApiKey",
				TFName:      "api_key",
				Type:        "String",
				SchemaType:  "String",
				Description: "",
				Required:    false,
				Optional:    false,
				Computed:    true,
				Sensitive:   true,
			},
		},
		{
			name: "computed_optional attribute",
			input: AttributeSpec{
				Name: "description",
				String: &TypeSpec{
					ComputedOptionalRequired: "computed_optional",
				},
			},
			expected: AttributeData{
				Name:        "description",
				FieldName:   "Description",
				TFName:      "description",
				Type:        "String",
				SchemaType:  "String",
				Description: "",
				Required:    false,
				Optional:    true,
				Computed:    true,
				Sensitive:   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertAttribute(tt.input)
			if result.Name != tt.expected.Name {
				t.Errorf("Name = %q, want %q", result.Name, tt.expected.Name)
			}
			if result.FieldName != tt.expected.FieldName {
				t.Errorf("FieldName = %q, want %q", result.FieldName, tt.expected.FieldName)
			}
			if result.Type != tt.expected.Type {
				t.Errorf("Type = %q, want %q", result.Type, tt.expected.Type)
			}
			if result.Required != tt.expected.Required {
				t.Errorf("Required = %v, want %v", result.Required, tt.expected.Required)
			}
			if result.Optional != tt.expected.Optional {
				t.Errorf("Optional = %v, want %v", result.Optional, tt.expected.Optional)
			}
			if result.Computed != tt.expected.Computed {
				t.Errorf("Computed = %v, want %v", result.Computed, tt.expected.Computed)
			}
			if result.Sensitive != tt.expected.Sensitive {
				t.Errorf("Sensitive = %v, want %v", result.Sensitive, tt.expected.Sensitive)
			}
		})
	}
}

func TestPrepareTemplateData(t *testing.T) {
	schema := SchemaSpec{
		Attributes: []AttributeSpec{
			{
				Name: "id",
				String: &TypeSpec{
					ComputedOptionalRequired: "computed",
				},
			},
			{
				Name: "name",
				String: &TypeSpec{
					ComputedOptionalRequired: "required",
				},
			},
			{
				Name: "description",
				String: &TypeSpec{
					ComputedOptionalRequired: "optional",
				},
			},
		},
	}

	data := prepareTemplateData("test_resource", schema)

	if data.ResourceName != "TestResource" {
		t.Errorf("ResourceName = %q, want %q", data.ResourceName, "TestResource")
	}
	if data.TypeName != "test_resource" {
		t.Errorf("TypeName = %q, want %q", data.TypeName, "test_resource")
	}
	if data.CreateMethod != "CreateTestResource" {
		t.Errorf("CreateMethod = %q, want %q", data.CreateMethod, "CreateTestResource")
	}
	if data.ReadMethod != "GetTestResource" {
		t.Errorf("ReadMethod = %q, want %q", data.ReadMethod, "GetTestResource")
	}

	// ID should be filtered out
	if len(data.Attributes) != 2 {
		t.Errorf("Expected 2 attributes (id filtered out), got %d", len(data.Attributes))
	}

	// Check first attribute (name)
	if data.Attributes[0].FieldName != "Name" {
		t.Errorf("First attribute FieldName = %q, want %q", data.Attributes[0].FieldName, "Name")
	}
	if !data.Attributes[0].Required {
		t.Errorf("First attribute should be required")
	}

	// Check second attribute (description)
	if data.Attributes[1].FieldName != "Description" {
		t.Errorf("Second attribute FieldName = %q, want %q", data.Attributes[1].FieldName, "Description")
	}
	if !data.Attributes[1].Optional {
		t.Errorf("Second attribute should be optional")
	}
}

func TestReadSpec(t *testing.T) {
	// Create a temporary spec file
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test_spec.json")

	spec := ProviderSpec{
		Provider: ProviderInfo{Name: "arcane"},
		Resources: []ResourceSpec{
			{
				Name: "environment",
				Schema: SchemaSpec{
					Attributes: []AttributeSpec{
						{
							Name: "name",
							String: &TypeSpec{
								ComputedOptionalRequired: "required",
							},
						},
					},
				},
			},
		},
		DataSources: []DataSourceSpec{
			{
				Name: "environment",
				Schema: SchemaSpec{
					Attributes: []AttributeSpec{
						{
							Name: "id",
							String: &TypeSpec{
								ComputedOptionalRequired: "optional",
							},
						},
					},
				},
			},
		},
	}

	// Write spec to temp file
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Failed to marshal spec: %v", err)
	}
	if err := os.WriteFile(specPath, data, 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Test reading the spec
	result, err := readSpec(specPath)
	if err != nil {
		t.Fatalf("readSpec failed: %v", err)
	}

	if result.Provider.Name != "arcane" {
		t.Errorf("Provider name = %q, want %q", result.Provider.Name, "arcane")
	}
	if len(result.Resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(result.Resources))
	}
	if len(result.DataSources) != 1 {
		t.Errorf("Expected 1 data source, got %d", len(result.DataSources))
	}
}

func TestReadSpecNotFound(t *testing.T) {
	_, err := readSpec("/nonexistent/path/spec.json")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "spec file not found") {
		t.Errorf("Expected 'spec file not found' error, got: %v", err)
	}
}

func TestGenerationHeader(t *testing.T) {
	code := "package provider\n\nfunc Test() {}"
	result := addGenerationHeader(code)

	if !strings.Contains(result, "DO NOT EDIT") {
		t.Error("Header should contain 'DO NOT EDIT'")
	}
	if !strings.Contains(result, "Code generated") {
		t.Error("Header should contain 'Code generated'")
	}
	if !strings.Contains(result, code) {
		t.Error("Result should contain original code")
	}
}

func TestLoadTemplates(t *testing.T) {
	// Create temporary template directory
	tmpDir := t.TempDir()

	// Write a simple test template
	tmplContent := `package provider

type {{.ResourceName}}Resource struct {}
`
	tmplPath := filepath.Join(tmpDir, "test.go.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write test template: %v", err)
	}

	// Test loading templates
	templates, err := loadTemplates(tmpDir)
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	// Check that template was loaded
	tmpl := templates.Lookup("test.go.tmpl")
	if tmpl == nil {
		t.Error("Template 'test.go.tmpl' not found")
	}
}

func TestGenerateResourceDryRun(t *testing.T) {
	// Create temporary template directory
	tmpDir := t.TempDir()

	// Write minimal resource template
	tmplContent := `package provider

type {{.ResourceName}}Resource struct {
	{{range .Attributes}}
	{{.FieldName}} types.{{.Type}}
	{{end}}
}
`
	tmplPath := filepath.Join(tmpDir, "resource.go.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write resource template: %v", err)
	}

	// Load templates
	templates, err := loadTemplates(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	// Create test resource spec
	res := ResourceSpec{
		Name: "test_resource",
		Schema: SchemaSpec{
			Attributes: []AttributeSpec{
				{
					Name: "name",
					String: &TypeSpec{
						ComputedOptionalRequired: "required",
					},
				},
			},
		},
	}

	// Test dry-run generation (shouldn't write files)
	outputDir := t.TempDir()
	err = generateResource(res, templates, outputDir, true)
	if err != nil {
		t.Fatalf("generateResource failed: %v", err)
	}

	// Verify no files were written
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("Failed to read output dir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("Dry run should not write files, but found %d files", len(entries))
	}
}

func TestGenerateResource(t *testing.T) {
	// Create temporary template directory
	tmpDir := t.TempDir()

	// Write minimal resource template
	tmplContent := `package provider

type {{.ResourceName}}Resource struct {
	{{range .Attributes}}
	{{.FieldName}} string
	{{end}}
}
`
	tmplPath := filepath.Join(tmpDir, "resource.go.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write resource template: %v", err)
	}

	// Load templates
	templates, err := loadTemplates(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	// Create test resource spec
	res := ResourceSpec{
		Name: "test_resource",
		Schema: SchemaSpec{
			Attributes: []AttributeSpec{
				{
					Name: "name",
					String: &TypeSpec{
						ComputedOptionalRequired: "required",
					},
				},
			},
		},
	}

	// Test actual generation
	outputDir := t.TempDir()
	err = generateResource(res, templates, outputDir, false)
	if err != nil {
		t.Fatalf("generateResource failed: %v", err)
	}

	// Verify file was created
	expectedFile := filepath.Join(outputDir, "test_resource_resource_generated.go")
	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	// Check content
	contentStr := string(content)
	if !strings.Contains(contentStr, "package provider") {
		t.Error("Generated file should contain 'package provider'")
	}
	if !strings.Contains(contentStr, "type TestResourceResource struct") {
		t.Error("Generated file should contain resource type definition")
	}
	if !strings.Contains(contentStr, "DO NOT EDIT") {
		t.Error("Generated file should contain generation header")
	}
}
