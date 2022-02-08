/*
Copyright 2022 The Kubernetes Authors.

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

/*
Copyright 2022 The Kubernetes Authors.
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

package helpers

import (
	"os"

	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"
)

type mod struct {
	path    string
	content []byte
}

func newMod(path string) (mod, error) {
	var m mod
	content, err := os.ReadFile(path) //#nosec
	if err != nil {
		return m, err
	}
	return mod{
		path:    path,
		content: content,
	}, nil
}

func (m mod) FindDependencyVersion(dependency string) (string, error) {
	f, err := modfile.Parse(m.path, m.content, nil)
	if err != nil {
		return "", err
	}

	var version string
	for _, entry := range f.Require {
		if entry.Mod.Path == dependency {
			version = entry.Mod.Version
			break
		}
	}
	if version == "" {
		return version, errors.Errorf("could not find required package: %s", dependency)
	}

	for _, entry := range f.Replace {
		if entry.New.Path == dependency && entry.New.Version != "" {
			version = entry.New.Version
		}
	}
	return version, nil
}
