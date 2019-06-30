/*
Copyright 2019 The Kubernetes Authors.

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

package ibmcloud

import (
	"os"
	"strings"
	"testing"
)

func TestGetSSHKeyFile(t *testing.T) {
	// default case
	homeDir, ok := os.LookupEnv("HOME")
	if !ok {
		t.Errorf("Unable to use HOME environment variable to find SSH key")
	}
	defaultKey := homeDir + "/.ssh/id_ibmcloud"
	targetKeyfile := getSSHKeyFile(homeDir)
	if 0 != strings.Compare(targetKeyfile, defaultKey) {
		t.Errorf("Unexpected output: %s, expect output: %s", targetKeyfile, defaultKey)
	}

	// custom case
	customKey := "examples/ibmcloud/mykey"
	err := os.Setenv("IBMCLOUD_HOST_SSH_PRIVATE_FILE", customKey)
	if err != nil {
		t.Errorf("Can not set environment variable IBMCLOUD_HOST_SSH_PRIVATE_FILE")
	}
	targetKeyfile = getSSHKeyFile(homeDir)
	if 0 != strings.Compare(targetKeyfile, customKey) {
		t.Errorf("Unexpected output: %s, expect output: %s", targetKeyfile, customKey)
	}
}
