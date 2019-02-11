/*
Copyright 2017 The Kubernetes Authors.

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

package connection

import (
	"context"
	"fmt"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-csi/csi-test/driver"
)

func createMockServer(t *testing.T) (
	*gomock.Controller,
	*driver.MockCSIDriver,
	*driver.MockIdentityServer,
	*driver.MockControllerServer,
	*driver.MockNodeServer,
	CSIConnection,
	error) {
	// Start the mock server
	mockController := gomock.NewController(t)
	identityServer := driver.NewMockIdentityServer(mockController)
	controllerServer := driver.NewMockControllerServer(mockController)
	nodeServer := driver.NewMockNodeServer(mockController)
	drv := driver.NewMockCSIDriver(&driver.MockCSIDriverServers{
		Identity:   identityServer,
		Controller: controllerServer,
		Node:       nodeServer,
	})
	drv.Start()

	// Create a client connection to it
	addr := drv.Address()
	csiConn, err := NewConnection(addr)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	return mockController, drv, identityServer, controllerServer, nodeServer, csiConn, nil
}

func TestIsAttachRequired(t *testing.T) {
	tests := []struct {
		name           string
		output         *csi.ControllerGetCapabilitiesResponse
		attachRequired bool
		injectError    bool
		expectError    bool
	}{
		{
			name: "success",
			output: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
							},
						},
					},
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
							},
						},
					},
				},
			},
			attachRequired: true,
			expectError:    false,
		},
		{
			name: "no publish",
			output: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
							},
						},
					},
				},
			},
			attachRequired: false,
			expectError:    false,
		},
		{
			name:        "gRPC error",
			output:      nil,
			injectError: true,
			expectError: true,
		},
		{
			name: "empty capabilities",
			output: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: nil,
			},
			attachRequired: false,
			expectError:    false,
		},
	}

	mockController, driver, _, controllerServer, _, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()
	defer csiConn.Close()

	for _, test := range tests {

		in := &csi.ControllerGetCapabilitiesRequest{}

		out := test.output
		var injectedErr error
		if test.injectError {
			injectedErr = fmt.Errorf("mock error")
		}

		// Setup expectation
		controllerServer.EXPECT().ControllerGetCapabilities(gomock.Any(), in).Return(out, injectedErr).Times(1)

		attachRequired, err := csiConn.IsAttachRequired(context.Background())
		if test.expectError && err == nil {
			t.Errorf("test %q: Expected error, got none", test.name)
		}
		if !test.expectError && err != nil {
			t.Errorf("test %q: got error: %v", test.name, err)
		}
		if err == nil && attachRequired != test.attachRequired {
			t.Errorf("expecting attachRequired == %t, got %t", test.attachRequired, attachRequired)
		}
	}
}
