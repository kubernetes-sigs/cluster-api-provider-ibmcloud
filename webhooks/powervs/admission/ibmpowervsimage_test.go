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

package powervs

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSImage_Default(t *testing.T) {
	g := NewWithT(t)
	image := &infrav1.IBMPowerVSImage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-image",
			Namespace: "default",
		},
		Spec: infrav1.IBMPowerVSImageSpec{
			ClusterName: "capi-cluster",
			Bucket:      "my-bucket",
			Object:      "my-image.ova",
			Region:      "us-south",
		},
	}
	// Default is a no-op; it should succeed and not modify the object.
	g.Expect((&IBMPowerVSImage{}).Default(context.Background(), image)).To(Succeed())
	g.Expect(image.Spec.ClusterName).To(Equal("capi-cluster"))
}

func TestIBMPowerVSImage_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		image   *infrav1.IBMPowerVSImage
		wantErr bool
	}{
		{
			name: "Should successfully validate a valid IBMPowerVSImage",
			image: &infrav1.IBMPowerVSImage{
				Spec: infrav1.IBMPowerVSImageSpec{
					ClusterName: "capi-cluster",
					Bucket:      "my-bucket",
					Object:      "my-image.ova",
					Region:      "us-south",
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			w := &IBMPowerVSImage{}
			warnings, err := w.ValidateCreate(context.Background(), tc.image)
			g.Expect(warnings).To(BeNil())
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestIBMPowerVSImage_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldObj  *infrav1.IBMPowerVSImage
		newObj  *infrav1.IBMPowerVSImage
		wantErr bool
	}{
		{
			name: "Should successfully validate an update to IBMPowerVSImage",
			oldObj: &infrav1.IBMPowerVSImage{
				Spec: infrav1.IBMPowerVSImageSpec{
					ClusterName: "capi-cluster",
					Bucket:      "my-bucket",
					Object:      "my-image-v1.ova",
					Region:      "us-south",
				},
			},
			newObj: &infrav1.IBMPowerVSImage{
				Spec: infrav1.IBMPowerVSImageSpec{
					ClusterName: "capi-cluster",
					Bucket:      "my-bucket",
					Object:      "my-image-v2.ova",
					Region:      "us-south",
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			w := &IBMPowerVSImage{}
			warnings, err := w.ValidateUpdate(context.Background(), tc.oldObj, tc.newObj)
			g.Expect(warnings).To(BeNil())
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestIBMPowerVSImage_ValidateDelete(t *testing.T) {
	g := NewWithT(t)
	image := &infrav1.IBMPowerVSImage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-image",
			Namespace: "default",
		},
		Spec: infrav1.IBMPowerVSImageSpec{
			ClusterName: "capi-cluster",
			Bucket:      "my-bucket",
			Object:      "my-image.ova",
			Region:      "us-south",
		},
	}
	w := &IBMPowerVSImage{}
	warnings, err := w.ValidateDelete(context.Background(), image)
	g.Expect(warnings).To(BeNil())
	g.Expect(err).NotTo(HaveOccurred())
}

func TestIBMPowerVSImage_create(t *testing.T) {
	tests := []struct {
		name    string
		image   *infrav1.IBMPowerVSImage
		wantErr bool
	}{
		{
			name: "Should successfully create a valid IBMPowerVSImage via testEnv",
			image: &infrav1.IBMPowerVSImage{
				Spec: infrav1.IBMPowerVSImageSpec{
					ClusterName: "capi-cluster",
					Bucket:      "my-bucket",
					Object:      "my-image.ova",
					Region:      "us-south",
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			img := tc.image.DeepCopy()
			img.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-image-",
				Namespace:    "default",
			}
			if err := testEnv.Create(ctx, img); (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIBMPowerVSImage_update(t *testing.T) {
	tests := []struct {
		name    string
		oldObj  *infrav1.IBMPowerVSImage
		newObj  *infrav1.IBMPowerVSImage
		wantErr bool
	}{
		{
			name: "Should successfully update an IBMPowerVSImage via testEnv",
			oldObj: &infrav1.IBMPowerVSImage{
				Spec: infrav1.IBMPowerVSImageSpec{
					ClusterName: "capi-cluster",
					Bucket:      "my-bucket",
					Object:      "my-image-v1.ova",
					Region:      "us-south",
				},
			},
			newObj: &infrav1.IBMPowerVSImage{
				Spec: infrav1.IBMPowerVSImageSpec{
					ClusterName: "capi-cluster",
					Bucket:      "my-bucket",
					Object:      "my-image-v2.ova",
					Region:      "us-south",
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			img := tc.oldObj.DeepCopy()
			img.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-image-",
				Namespace:    "default",
			}
			if err := testEnv.Create(ctx, img); err != nil {
				t.Errorf("failed to create image: %v", err)
			}
			img.Spec = tc.newObj.Spec
			if err := testEnv.Update(ctx, img); (err != nil) != tc.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
