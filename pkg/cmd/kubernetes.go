/*
Copyright 2017 The Kedge Authors All rights reserved.

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

package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kedgeproject/kedge/pkg/spec"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

// Generate Kubernetes Artifacts and either writes to file
// or uses kubectl to deploy.
// TODO: Refactor into two separate functions (remove `generate bool`).
func CreateKubernetesArtifacts(paths []string, generate bool, args ...string) error {

	files, err := GetAllYAMLFiles(paths)
	if err != nil {
		return errors.Wrap(err, "unable to get YAML files")
	}

	inputs, err := getApplicationsFromFiles(files)
	if err != nil {
		return errors.Wrap(err, "unable to get kedge definitions from input files")
	}

	for _, input := range inputs {

		ros, extraResources, err := spec.CoreOperations(input.data)
		if err != nil {
			return errors.Wrap(err, "unable to perform controller operations")
		}

		for _, runtimeObject := range ros {

			// Unmarshal said object
			data, err := yaml.Marshal(runtimeObject)
			if err != nil {
				return errors.Wrap(err, "failed to marshal object")
			}

			// Write to file if generate = true
			if generate {
				err = writeObject(data)
				if err != nil {
					return errors.Wrap(err, "failed to write object")
				}
			} else {
				// We need to add "-f -" at the end of the command passed to us to
				// pass the generated files.
				// e.g. If the command and arguments are "apply --namespace staging", then the
				// final command becomes "kubectl apply --namespace staging -f -"
				arguments := append(args, "-f", "-")
				err = runKubectl(arguments, data)
				if err != nil {
					return errors.Wrap(err, "kubectl error")
				}
			}

		}

		for _, file := range extraResources {
			// change the file name to absolute file name
			file = findAbsPath(input.fileName, file)

			if generate {
				data, err := ioutil.ReadFile(file)
				if err != nil {
					return errors.Wrap(err, "file reading failed")
				}
				err = writeObject(data)
				if err != nil {
					return errors.Wrap(err, "failed to write object")
				}
			} else {

				// We need to add "-f absolute-filename" at the end of the command passed to us to
				// pass the generated files.
				// e.g. If the command and arguments are "apply --namespace staging", then the
				// final command becomes "kubectl apply --namespace staging -f absolute-filename"
				arguments := append(args, "-f", file)
				err = runKubectl(arguments, nil)
				if err != nil {
					return errors.Wrap(err, "kubectl error")
				}
			}
		}
	}
	return nil
}

func runKubectl(args []string, data []byte) error {
	cmd := exec.Command("kubectl", args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "can't get stdinPipe for kubectl")
	}

	go func() {
		defer stdin.Close()
		_, err := io.WriteString(stdin, string(data))
		if err != nil {
			fmt.Printf("can't write to stdin %v\n", err)
		}
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("%s", string(out))
		return errors.Wrap(err, "failed to execute command")
	}
	fmt.Printf("%s", string(out))
	return nil
}

func writeObject(data []byte) error {
	_, err := fmt.Fprintln(os.Stdout, "---")
	if err != nil {
		return errors.Wrap(err, "could not print to STDOUT")
	}

	_, err = os.Stdout.Write(data)
	return errors.Wrap(err, "could not write to STDOUT")
}

func findAbsPath(baseFilePath, path string) string {
	// TODO: if the baseFilePath is empty then just take the
	// pwd as basefilePath, here we will force user to
	// use the kedge binary from the directory that has files
	// otherwise there is no way of knowing where the files will be
	// this condition will happen when we add support for reading from the stdin
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(baseFilePath), path)
}
