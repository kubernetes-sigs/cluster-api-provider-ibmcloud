/*
Copyright 2023 The Kubernetes Authors.

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

package record

import (
	"testing"

	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cgrecord "k8s.io/client-go/tools/record"
)

type fakeObject struct {
}

func (f *fakeObject) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (f *fakeObject) DeepCopyObject() runtime.Object {
	return f
}

func TestEvent(t *testing.T) {
	testCases := []struct {
		name          string
		object        runtime.Object
		reason        string
		message       string
		expectedEvent string
	}{
		{
			name:          "format reason",
			object:        &fakeObject{},
			reason:        "reason",
			message:       "message",
			expectedEvent: "Normal Reason message",
		},
		{
			name:          "format long reason",
			object:        &fakeObject{},
			reason:        "this is a very long reason",
			message:       "message",
			expectedEvent: "Normal This Is A Very Long Reason message",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := defaultRecorder.(*cgrecord.FakeRecorder)
			recorder.Events = make(chan string, 1)

			Event(tc.object, tc.reason, tc.message)

			require.Equal(t, tc.expectedEvent, <-recorder.Events)
		})
	}
}

func TestEventf(t *testing.T) {
	testCases := []struct {
		name          string
		object        runtime.Object
		reason        string
		message       string
		args          []interface{}
		expectedEvent string
	}{
		{
			name:          "format reason",
			object:        &fakeObject{},
			reason:        "reason",
			message:       "message %s",
			args:          []interface{}{"arg1"},
			expectedEvent: "Normal Reason message arg1",
		},
		{
			name:          "format long reason",
			object:        &fakeObject{},
			reason:        "this is a very long reason",
			message:       "message %s",
			args:          []interface{}{"arg1"},
			expectedEvent: "Normal This Is A Very Long Reason message arg1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := defaultRecorder.(*cgrecord.FakeRecorder)
			recorder.Events = make(chan string, 1)

			Eventf(tc.object, tc.reason, tc.message, tc.args...)

			require.Equal(t, tc.expectedEvent, <-recorder.Events)
		})
	}
}

func TestWarn(t *testing.T) {
	testCases := []struct {
		name          string
		object        runtime.Object
		reason        string
		message       string
		expectedEvent string
	}{
		{
			name:          "format reason",
			object:        &fakeObject{},
			reason:        "reason",
			message:       "message",
			expectedEvent: "Warning Reason message",
		},
		{
			name:          "format long reason",
			object:        &fakeObject{},
			reason:        "this is a very long reason",
			message:       "message",
			expectedEvent: "Warning This Is A Very Long Reason message",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := defaultRecorder.(*cgrecord.FakeRecorder)
			recorder.Events = make(chan string, 1)

			Warn(tc.object, tc.reason, tc.message)

			require.Equal(t, tc.expectedEvent, <-recorder.Events)
		})
	}
}

func TestWarnf(t *testing.T) {
	testCases := []struct {
		name          string
		object        runtime.Object
		reason        string
		message       string
		args          []interface{}
		expectedEvent string
	}{
		{
			name:          "format reason",
			object:        &fakeObject{},
			reason:        "reason",
			message:       "message %s",
			args:          []interface{}{"arg1"},
			expectedEvent: "Warning Reason message arg1",
		},
		{
			name:          "format long reason",
			object:        &fakeObject{},
			reason:        "this is a very long reason",
			message:       "message %s",
			args:          []interface{}{"arg1"},
			expectedEvent: "Warning This Is A Very Long Reason message arg1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := defaultRecorder.(*cgrecord.FakeRecorder)
			recorder.Events = make(chan string, 1)

			Warnf(tc.object, tc.reason, tc.message, tc.args...)

			require.Equal(t, tc.expectedEvent, <-recorder.Events)
		})
	}
}
