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

package v1beta1

import (
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/defaulting"
)

func TestVPCMachine_default(t *testing.T) {
	g := NewWithT(t)
	vpcMachine := &IBMVPCMachine{ObjectMeta: metav1.ObjectMeta{Name: "capi-machine", Namespace: "default"}, Spec: IBMVPCMachineSpec{Image: "capi-image-id"}}
	t.Run("Defaults for IBMVPCMachine", defaulting.DefaultValidateTest(vpcMachine))
	vpcMachine.Default()
	g.Expect(vpcMachine.Spec.Profile).To(BeEquivalentTo("bx2-2x8"))
}

func TestIBMVPCMachine_create(t *testing.T) {
	tests := []struct {
		name       string
		vpcMachine *IBMVPCMachine
		wantErr    bool
	}{
		{
			name: "Should fail to validate IBMVPCMachineSpec - no Image or Imagref in Spec",
			vpcMachine: &IBMVPCMachine{
				Spec: IBMVPCMachineSpec{
					Name: "capi-vpc",
				},
			},
			wantErr: true,
		},
		{
			name: "Should successfully validate IBMVPCMachineSpec - valid spec",
			vpcMachine: &IBMVPCMachine{
				Spec: IBMVPCMachineSpec{
					Name: "capi-vpc",
					ImageRef: &IBMVPCResourceReference{
						ID: core.StringPtr("capi-image"),
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			machine := tc.vpcMachine.DeepCopy()
			machine.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-machine-",
				Namespace:    "default",
			}

			if err := testEnv.Create(ctx, machine); (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIBMVPCMachine_update(t *testing.T) {
	tests := []struct {
		name          string
		oldVPCMachine *IBMVPCMachine
		newVPCMachine *IBMVPCMachine
		wantErr       bool
	}{
		{
			name: "Should fail to update IBMVPCMachine with no Image or ImageRef",
			oldVPCMachine: &IBMVPCMachine{
				Spec: IBMVPCMachineSpec{
					Name: "capi-vpc",
					ImageRef: &IBMVPCResourceReference{
						ID: core.StringPtr("capi-image"),
					},
				},
			},
			newVPCMachine: &IBMVPCMachine{
				Spec: IBMVPCMachineSpec{
					Name: "capi-vpc",
				},
			},
			wantErr: true,
		},
		{
			name: "Should successfully update IBMVPCMachine",
			oldVPCMachine: &IBMVPCMachine{
				Spec: IBMVPCMachineSpec{
					Name:  "capi-vpc",
					Image: "capi-image",
				},
			},
			newVPCMachine: &IBMVPCMachine{
				Spec: IBMVPCMachineSpec{
					Name: "capi-vpc",
					ImageRef: &IBMVPCResourceReference{
						ID: core.StringPtr("capi-image"),
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			machine := tc.oldVPCMachine.DeepCopy()
			machine.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-machine-",
				Namespace:    "default",
			}
			if err := testEnv.Create(ctx, machine); err != nil {
				t.Errorf("failed to create machine: %v", err)
			}
			machine.Spec = tc.newVPCMachine.Spec
			if err := testEnv.Update(ctx, machine); (err != nil) != tc.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
