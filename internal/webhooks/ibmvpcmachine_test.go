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

package webhooks

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"

	. "github.com/onsi/gomega"
)

func TestVPCMachine_default(t *testing.T) {
	g := NewWithT(t)
	vpcMachine := &infrav1.IBMVPCMachine{ObjectMeta: metav1.ObjectMeta{Name: "capi-machine", Namespace: "default"}}
	g.Expect((&IBMVPCMachine{}).Default(context.Background(), vpcMachine)).ToNot(HaveOccurred())
	g.Expect(vpcMachine.Spec.Profile).To(BeEquivalentTo("bx2-2x8"))
}

func TestIBMVPCMachine_Create(t *testing.T) {
	tests := []struct {
		name    string
		machine *infrav1.IBMVPCMachine
		wantErr bool
	}{
		{
			name: "Create a IBMVPCMachine with valid SizeGiB BootVolume",
			machine: &infrav1.IBMVPCMachine{
				Spec: infrav1.IBMVPCMachineSpec{
					BootVolume: &infrav1.VPCVolume{
						SizeGiB: 10,
					},
					Image: &infrav1.IBMVPCResourceReference{},
				},
			},
			wantErr: false,
		},
		{
			name: "Create a IBMVPCMachine with invalid SizeGiB BootVolume",
			machine: &infrav1.IBMVPCMachine{
				Spec: infrav1.IBMVPCMachineSpec{
					BootVolume: &infrav1.VPCVolume{
						SizeGiB: 1,
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := tt.machine.DeepCopy()
			machine.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "machine-",
				Namespace:    "default",
			}
			ctx := context.TODO()
			if err := testEnv.Create(ctx, machine); (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
