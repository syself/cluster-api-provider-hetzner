/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// TestStateEnumsMatchConstants checks that the CRD enum of each state field lists exactly the
// non-empty constants of that state. The controllers write these states into the status. A
// constant that is missing from the enum makes the API server reject that status write.
// The constants are read from the Go source of this package, so a new constant is found
// without changing this test.
func TestStateEnumsMatchConstants(t *testing.T) {
	constants := stringConstantsInPackage(t)

	tests := []struct {
		name string
		// constants selects the constants of the state.
		constants func(c stringConstant) bool
		crdFile   string
		// fieldPath is the path of the state field in the v1beta2 schema.
		fieldPath []string
	}{
		{
			name:      "HCloudMachine bootState",
			constants: func(c stringConstant) bool { return c.typeName == "HCloudBootState" },
			crdFile:   "infrastructure.cluster.x-k8s.io_hcloudmachines.yaml",
			fieldPath: []string{"status", "bootState"},
		},
		{
			name:      "HetznerBareMetalHost provisioningState",
			constants: func(c stringConstant) bool { return c.typeName == "ProvisioningState" },
			crdFile:   "infrastructure.cluster.x-k8s.io_hetznerbaremetalhosts.yaml",
			fieldPath: []string{"status", "provisioningState"},
		},
		{
			name:      "HetznerBareMetalHost errorType",
			constants: func(c stringConstant) bool { return c.typeName == "ErrorType" },
			crdFile:   "infrastructure.cluster.x-k8s.io_hetznerbaremetalhosts.yaml",
			fieldPath: []string{"status", "errorType"},
		},
		{
			// The controller sends a reboot via SSH or via the Robot API as a software or hardware
			// reboot (setOngoingReboot and rebootTypeAfterTimeout in pkg/services/baremetal/host).
			// The other RebootType constants only appear in status.rebootTypes, the list of reboot
			// types that the Robot API offers for the host.
			name: "HetznerBareMetalHost ongoingReboot type",
			constants: func(c stringConstant) bool {
				return c.name == "RebootTypeSSH" || c.name == "RebootTypeSoftware" || c.name == "RebootTypeHardware"
			},
			crdFile:   "infrastructure.cluster.x-k8s.io_hetznerbaremetalhosts.yaml",
			fieldPath: []string{"status", "ongoingReboot", "type"},
		},
		{
			name:      "HCloudRemediation phase",
			constants: isRemediationPhase,
			crdFile:   "infrastructure.cluster.x-k8s.io_hcloudremediations.yaml",
			fieldPath: []string{"status", "phase"},
		},
		{
			name:      "HetznerBareMetalRemediation phase",
			constants: isRemediationPhase,
			crdFile:   "infrastructure.cluster.x-k8s.io_hetznerbaremetalremediations.yaml",
			fieldPath: []string{"status", "phase"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var want []string
			for _, c := range constants {
				// An empty state is not written, because the state fields use omitempty.
				if tc.constants(c) && c.value != "" {
					want = append(want, c.value)
				}
			}
			require.NotEmpty(t, want, "found no constants for this state")

			require.ElementsMatch(t, want, crdEnum(t, tc.crdFile, tc.fieldPath),
				"the Enum marker of the state does not match its constants. Update the marker and run make generate-manifests")
		})
	}
}

// isRemediationPhase selects the untyped Phase constants in remediation_types.go.
func isRemediationPhase(c stringConstant) bool {
	return c.file == "remediation_types.go" && c.typeName == "" && strings.HasPrefix(c.name, "Phase")
}

type stringConstant struct {
	file     string
	name     string
	typeName string
	value    string
}

// stringConstantsInPackage returns every string constant that the non-test Go files of this
// package declare.
func stringConstantsInPackage(t *testing.T) []stringConstant {
	t.Helper()

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	var constants []stringConstant
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || strings.HasPrefix(file, "zz_generated") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		require.NoError(t, err)

		for _, decl := range parsed.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec := spec.(*ast.ValueSpec)
				typeName := ""
				if ident, ok := valueSpec.Type.(*ast.Ident); ok {
					typeName = ident.Name
				}
				for i, name := range valueSpec.Names {
					if i >= len(valueSpec.Values) {
						continue
					}
					literal, ok := valueSpec.Values[i].(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						continue
					}
					value, err := strconv.Unquote(literal.Value)
					require.NoError(t, err)
					constants = append(constants, stringConstant{
						file:     file,
						name:     name.Name,
						typeName: typeName,
						value:    value,
					})
				}
			}
		}
	}
	return constants
}

// crdEnum returns the enum of a field in the v1beta2 schema of a generated CRD.
func crdEnum(t *testing.T, crdFile string, fieldPath []string) []string {
	t.Helper()

	schema := crdFieldSchema(t, crdFile, fieldPath)
	values := make([]string, 0, len(schema.Enum))
	for _, raw := range schema.Enum {
		var value string
		require.NoError(t, json.Unmarshal(raw.Raw, &value))
		values = append(values, value)
	}
	return values
}

// crdFieldSchema returns the schema of a field in the v1beta2 schema of a generated CRD.
func crdFieldSchema(t *testing.T, crdFile string, fieldPath []string) *apiextensionsv1.JSONSchemaProps {
	t.Helper()

	data, err := fs.ReadFile(os.DirFS(filepath.Join("..", "..", "config", "crd", "bases")), crdFile)
	require.NoError(t, err)

	var crd apiextensionsv1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal(data, &crd))

	var schema *apiextensionsv1.JSONSchemaProps
	for _, version := range crd.Spec.Versions {
		if version.Name == GroupVersion.Version {
			schema = version.Schema.OpenAPIV3Schema
		}
	}
	require.NotNil(t, schema, "CRD has no %s schema", GroupVersion.Version)

	for _, name := range fieldPath {
		property, ok := schema.Properties[name]
		require.True(t, ok, "CRD has no field %s", strings.Join(fieldPath, "."))
		schema = &property
	}
	return schema
}
