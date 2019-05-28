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

package cloud

// Config is designed for different cloud providers configuration
type Config struct {
	Clouds Clouds `yaml:"clouds,omitempty"`
}

// Clouds can be different cloud providers for extension purpose
type Clouds struct {
	IBMCloud IBMCloudConfig `yaml:"ibmcloud,omitempty"`
}

// IBMCloudConfig holds ibm cloud provider required configurations
type IBMCloudConfig struct {
	Auth AuthConfig `yaml:"auth,omitempty"`
}

// AuthConfig is mounted into controller pod for clouds authentication
type AuthConfig struct {
	APIUserName       string `yaml:"apiUserName,omitempty"`
	AuthenticationKey string `yaml:"authenticationKey,omitempty"`
}
