/*
Copyright The Kubernetes Authors.

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
// Code generated by MockGen. DO NOT EDIT.
// Source: ./vpc.go
//
// Generated by this command:
//
//	mockgen -source=./vpc.go -destination=./mock/vpc_generated.go -package=mock
//

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	core "github.com/IBM/go-sdk-core/v5/core"
	vpcv1 "github.com/IBM/vpc-go-sdk/vpcv1"
	gomock "go.uber.org/mock/gomock"
)

// MockVpc is a mock of Vpc interface.
type MockVpc struct {
	ctrl     *gomock.Controller
	recorder *MockVpcMockRecorder
}

// MockVpcMockRecorder is the mock recorder for MockVpc.
type MockVpcMockRecorder struct {
	mock *MockVpc
}

// NewMockVpc creates a new mock instance.
func NewMockVpc(ctrl *gomock.Controller) *MockVpc {
	mock := &MockVpc{ctrl: ctrl}
	mock.recorder = &MockVpcMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVpc) EXPECT() *MockVpcMockRecorder {
	return m.recorder
}

// CreateImage mocks base method.
func (m *MockVpc) CreateImage(options *vpcv1.CreateImageOptions) (*vpcv1.Image, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateImage", options)
	ret0, _ := ret[0].(*vpcv1.Image)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateImage indicates an expected call of CreateImage.
func (mr *MockVpcMockRecorder) CreateImage(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateImage", reflect.TypeOf((*MockVpc)(nil).CreateImage), options)
}

// CreateInstance mocks base method.
func (m *MockVpc) CreateInstance(options *vpcv1.CreateInstanceOptions) (*vpcv1.Instance, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateInstance", options)
	ret0, _ := ret[0].(*vpcv1.Instance)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateInstance indicates an expected call of CreateInstance.
func (mr *MockVpcMockRecorder) CreateInstance(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateInstance", reflect.TypeOf((*MockVpc)(nil).CreateInstance), options)
}

// CreateLoadBalancer mocks base method.
func (m *MockVpc) CreateLoadBalancer(options *vpcv1.CreateLoadBalancerOptions) (*vpcv1.LoadBalancer, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateLoadBalancer", options)
	ret0, _ := ret[0].(*vpcv1.LoadBalancer)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateLoadBalancer indicates an expected call of CreateLoadBalancer.
func (mr *MockVpcMockRecorder) CreateLoadBalancer(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateLoadBalancer", reflect.TypeOf((*MockVpc)(nil).CreateLoadBalancer), options)
}

// CreateLoadBalancerPoolMember mocks base method.
func (m *MockVpc) CreateLoadBalancerPoolMember(options *vpcv1.CreateLoadBalancerPoolMemberOptions) (*vpcv1.LoadBalancerPoolMember, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateLoadBalancerPoolMember", options)
	ret0, _ := ret[0].(*vpcv1.LoadBalancerPoolMember)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateLoadBalancerPoolMember indicates an expected call of CreateLoadBalancerPoolMember.
func (mr *MockVpcMockRecorder) CreateLoadBalancerPoolMember(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateLoadBalancerPoolMember", reflect.TypeOf((*MockVpc)(nil).CreateLoadBalancerPoolMember), options)
}

// CreatePublicGateway mocks base method.
func (m *MockVpc) CreatePublicGateway(options *vpcv1.CreatePublicGatewayOptions) (*vpcv1.PublicGateway, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreatePublicGateway", options)
	ret0, _ := ret[0].(*vpcv1.PublicGateway)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreatePublicGateway indicates an expected call of CreatePublicGateway.
func (mr *MockVpcMockRecorder) CreatePublicGateway(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreatePublicGateway", reflect.TypeOf((*MockVpc)(nil).CreatePublicGateway), options)
}

// CreateSecurityGroup mocks base method.
func (m *MockVpc) CreateSecurityGroup(options *vpcv1.CreateSecurityGroupOptions) (*vpcv1.SecurityGroup, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateSecurityGroup", options)
	ret0, _ := ret[0].(*vpcv1.SecurityGroup)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateSecurityGroup indicates an expected call of CreateSecurityGroup.
func (mr *MockVpcMockRecorder) CreateSecurityGroup(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateSecurityGroup", reflect.TypeOf((*MockVpc)(nil).CreateSecurityGroup), options)
}

// CreateSecurityGroupRule mocks base method.
func (m *MockVpc) CreateSecurityGroupRule(options *vpcv1.CreateSecurityGroupRuleOptions) (vpcv1.SecurityGroupRuleIntf, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateSecurityGroupRule", options)
	ret0, _ := ret[0].(vpcv1.SecurityGroupRuleIntf)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateSecurityGroupRule indicates an expected call of CreateSecurityGroupRule.
func (mr *MockVpcMockRecorder) CreateSecurityGroupRule(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateSecurityGroupRule", reflect.TypeOf((*MockVpc)(nil).CreateSecurityGroupRule), options)
}

// CreateSubnet mocks base method.
func (m *MockVpc) CreateSubnet(options *vpcv1.CreateSubnetOptions) (*vpcv1.Subnet, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateSubnet", options)
	ret0, _ := ret[0].(*vpcv1.Subnet)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateSubnet indicates an expected call of CreateSubnet.
func (mr *MockVpcMockRecorder) CreateSubnet(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateSubnet", reflect.TypeOf((*MockVpc)(nil).CreateSubnet), options)
}

// CreateVPC mocks base method.
func (m *MockVpc) CreateVPC(options *vpcv1.CreateVPCOptions) (*vpcv1.VPC, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateVPC", options)
	ret0, _ := ret[0].(*vpcv1.VPC)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateVPC indicates an expected call of CreateVPC.
func (mr *MockVpcMockRecorder) CreateVPC(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateVPC", reflect.TypeOf((*MockVpc)(nil).CreateVPC), options)
}

// DeleteInstance mocks base method.
func (m *MockVpc) DeleteInstance(options *vpcv1.DeleteInstanceOptions) (*core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteInstance", options)
	ret0, _ := ret[0].(*core.DetailedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteInstance indicates an expected call of DeleteInstance.
func (mr *MockVpcMockRecorder) DeleteInstance(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteInstance", reflect.TypeOf((*MockVpc)(nil).DeleteInstance), options)
}

// DeleteLoadBalancer mocks base method.
func (m *MockVpc) DeleteLoadBalancer(options *vpcv1.DeleteLoadBalancerOptions) (*core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteLoadBalancer", options)
	ret0, _ := ret[0].(*core.DetailedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteLoadBalancer indicates an expected call of DeleteLoadBalancer.
func (mr *MockVpcMockRecorder) DeleteLoadBalancer(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteLoadBalancer", reflect.TypeOf((*MockVpc)(nil).DeleteLoadBalancer), options)
}

// DeleteLoadBalancerPoolMember mocks base method.
func (m *MockVpc) DeleteLoadBalancerPoolMember(options *vpcv1.DeleteLoadBalancerPoolMemberOptions) (*core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteLoadBalancerPoolMember", options)
	ret0, _ := ret[0].(*core.DetailedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteLoadBalancerPoolMember indicates an expected call of DeleteLoadBalancerPoolMember.
func (mr *MockVpcMockRecorder) DeleteLoadBalancerPoolMember(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteLoadBalancerPoolMember", reflect.TypeOf((*MockVpc)(nil).DeleteLoadBalancerPoolMember), options)
}

// DeletePublicGateway mocks base method.
func (m *MockVpc) DeletePublicGateway(options *vpcv1.DeletePublicGatewayOptions) (*core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeletePublicGateway", options)
	ret0, _ := ret[0].(*core.DetailedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeletePublicGateway indicates an expected call of DeletePublicGateway.
func (mr *MockVpcMockRecorder) DeletePublicGateway(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeletePublicGateway", reflect.TypeOf((*MockVpc)(nil).DeletePublicGateway), options)
}

// DeleteSecurityGroup mocks base method.
func (m *MockVpc) DeleteSecurityGroup(options *vpcv1.DeleteSecurityGroupOptions) (*core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteSecurityGroup", options)
	ret0, _ := ret[0].(*core.DetailedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteSecurityGroup indicates an expected call of DeleteSecurityGroup.
func (mr *MockVpcMockRecorder) DeleteSecurityGroup(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteSecurityGroup", reflect.TypeOf((*MockVpc)(nil).DeleteSecurityGroup), options)
}

// DeleteSubnet mocks base method.
func (m *MockVpc) DeleteSubnet(options *vpcv1.DeleteSubnetOptions) (*core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteSubnet", options)
	ret0, _ := ret[0].(*core.DetailedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteSubnet indicates an expected call of DeleteSubnet.
func (mr *MockVpcMockRecorder) DeleteSubnet(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteSubnet", reflect.TypeOf((*MockVpc)(nil).DeleteSubnet), options)
}

// DeleteVPC mocks base method.
func (m *MockVpc) DeleteVPC(options *vpcv1.DeleteVPCOptions) (*core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteVPC", options)
	ret0, _ := ret[0].(*core.DetailedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteVPC indicates an expected call of DeleteVPC.
func (mr *MockVpcMockRecorder) DeleteVPC(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteVPC", reflect.TypeOf((*MockVpc)(nil).DeleteVPC), options)
}

// GetImage mocks base method.
func (m *MockVpc) GetImage(options *vpcv1.GetImageOptions) (*vpcv1.Image, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetImage", options)
	ret0, _ := ret[0].(*vpcv1.Image)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetImage indicates an expected call of GetImage.
func (mr *MockVpcMockRecorder) GetImage(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetImage", reflect.TypeOf((*MockVpc)(nil).GetImage), options)
}

// GetImageByName mocks base method.
func (m *MockVpc) GetImageByName(imageName string) (*vpcv1.Image, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetImageByName", imageName)
	ret0, _ := ret[0].(*vpcv1.Image)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetImageByName indicates an expected call of GetImageByName.
func (mr *MockVpcMockRecorder) GetImageByName(imageName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetImageByName", reflect.TypeOf((*MockVpc)(nil).GetImageByName), imageName)
}

// GetInstance mocks base method.
func (m *MockVpc) GetInstance(options *vpcv1.GetInstanceOptions) (*vpcv1.Instance, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInstance", options)
	ret0, _ := ret[0].(*vpcv1.Instance)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetInstance indicates an expected call of GetInstance.
func (mr *MockVpcMockRecorder) GetInstance(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInstance", reflect.TypeOf((*MockVpc)(nil).GetInstance), options)
}

// GetInstanceProfile mocks base method.
func (m *MockVpc) GetInstanceProfile(options *vpcv1.GetInstanceProfileOptions) (*vpcv1.InstanceProfile, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInstanceProfile", options)
	ret0, _ := ret[0].(*vpcv1.InstanceProfile)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetInstanceProfile indicates an expected call of GetInstanceProfile.
func (mr *MockVpcMockRecorder) GetInstanceProfile(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInstanceProfile", reflect.TypeOf((*MockVpc)(nil).GetInstanceProfile), options)
}

// GetLoadBalancer mocks base method.
func (m *MockVpc) GetLoadBalancer(options *vpcv1.GetLoadBalancerOptions) (*vpcv1.LoadBalancer, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLoadBalancer", options)
	ret0, _ := ret[0].(*vpcv1.LoadBalancer)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetLoadBalancer indicates an expected call of GetLoadBalancer.
func (mr *MockVpcMockRecorder) GetLoadBalancer(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLoadBalancer", reflect.TypeOf((*MockVpc)(nil).GetLoadBalancer), options)
}

// GetLoadBalancerByName mocks base method.
func (m *MockVpc) GetLoadBalancerByName(loadBalancerName string) (*vpcv1.LoadBalancer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLoadBalancerByName", loadBalancerName)
	ret0, _ := ret[0].(*vpcv1.LoadBalancer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLoadBalancerByName indicates an expected call of GetLoadBalancerByName.
func (mr *MockVpcMockRecorder) GetLoadBalancerByName(loadBalancerName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLoadBalancerByName", reflect.TypeOf((*MockVpc)(nil).GetLoadBalancerByName), loadBalancerName)
}

// GetSecurityGroup mocks base method.
func (m *MockVpc) GetSecurityGroup(options *vpcv1.GetSecurityGroupOptions) (*vpcv1.SecurityGroup, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSecurityGroup", options)
	ret0, _ := ret[0].(*vpcv1.SecurityGroup)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetSecurityGroup indicates an expected call of GetSecurityGroup.
func (mr *MockVpcMockRecorder) GetSecurityGroup(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSecurityGroup", reflect.TypeOf((*MockVpc)(nil).GetSecurityGroup), options)
}

// GetSecurityGroupByName mocks base method.
func (m *MockVpc) GetSecurityGroupByName(name string) (*vpcv1.SecurityGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSecurityGroupByName", name)
	ret0, _ := ret[0].(*vpcv1.SecurityGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSecurityGroupByName indicates an expected call of GetSecurityGroupByName.
func (mr *MockVpcMockRecorder) GetSecurityGroupByName(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSecurityGroupByName", reflect.TypeOf((*MockVpc)(nil).GetSecurityGroupByName), name)
}

// GetSecurityGroupRule mocks base method.
func (m *MockVpc) GetSecurityGroupRule(options *vpcv1.GetSecurityGroupRuleOptions) (vpcv1.SecurityGroupRuleIntf, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSecurityGroupRule", options)
	ret0, _ := ret[0].(vpcv1.SecurityGroupRuleIntf)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetSecurityGroupRule indicates an expected call of GetSecurityGroupRule.
func (mr *MockVpcMockRecorder) GetSecurityGroupRule(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSecurityGroupRule", reflect.TypeOf((*MockVpc)(nil).GetSecurityGroupRule), options)
}

// GetSubnet mocks base method.
func (m *MockVpc) GetSubnet(arg0 *vpcv1.GetSubnetOptions) (*vpcv1.Subnet, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSubnet", arg0)
	ret0, _ := ret[0].(*vpcv1.Subnet)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetSubnet indicates an expected call of GetSubnet.
func (mr *MockVpcMockRecorder) GetSubnet(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSubnet", reflect.TypeOf((*MockVpc)(nil).GetSubnet), arg0)
}

// GetSubnetAddrPrefix mocks base method.
func (m *MockVpc) GetSubnetAddrPrefix(vpcID, zone string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSubnetAddrPrefix", vpcID, zone)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSubnetAddrPrefix indicates an expected call of GetSubnetAddrPrefix.
func (mr *MockVpcMockRecorder) GetSubnetAddrPrefix(vpcID, zone any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSubnetAddrPrefix", reflect.TypeOf((*MockVpc)(nil).GetSubnetAddrPrefix), vpcID, zone)
}

// GetSubnetPublicGateway mocks base method.
func (m *MockVpc) GetSubnetPublicGateway(options *vpcv1.GetSubnetPublicGatewayOptions) (*vpcv1.PublicGateway, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSubnetPublicGateway", options)
	ret0, _ := ret[0].(*vpcv1.PublicGateway)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetSubnetPublicGateway indicates an expected call of GetSubnetPublicGateway.
func (mr *MockVpcMockRecorder) GetSubnetPublicGateway(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSubnetPublicGateway", reflect.TypeOf((*MockVpc)(nil).GetSubnetPublicGateway), options)
}

// GetVPC mocks base method.
func (m *MockVpc) GetVPC(arg0 *vpcv1.GetVPCOptions) (*vpcv1.VPC, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVPC", arg0)
	ret0, _ := ret[0].(*vpcv1.VPC)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetVPC indicates an expected call of GetVPC.
func (mr *MockVpcMockRecorder) GetVPC(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVPC", reflect.TypeOf((*MockVpc)(nil).GetVPC), arg0)
}

// GetVPCByName mocks base method.
func (m *MockVpc) GetVPCByName(vpcName string) (*vpcv1.VPC, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVPCByName", vpcName)
	ret0, _ := ret[0].(*vpcv1.VPC)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVPCByName indicates an expected call of GetVPCByName.
func (mr *MockVpcMockRecorder) GetVPCByName(vpcName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVPCByName", reflect.TypeOf((*MockVpc)(nil).GetVPCByName), vpcName)
}

// GetVPCSubnetByName mocks base method.
func (m *MockVpc) GetVPCSubnetByName(subnetName string) (*vpcv1.Subnet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVPCSubnetByName", subnetName)
	ret0, _ := ret[0].(*vpcv1.Subnet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVPCSubnetByName indicates an expected call of GetVPCSubnetByName.
func (mr *MockVpcMockRecorder) GetVPCSubnetByName(subnetName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVPCSubnetByName", reflect.TypeOf((*MockVpc)(nil).GetVPCSubnetByName), subnetName)
}

// ListImages mocks base method.
func (m *MockVpc) ListImages(options *vpcv1.ListImagesOptions) (*vpcv1.ImageCollection, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListImages", options)
	ret0, _ := ret[0].(*vpcv1.ImageCollection)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListImages indicates an expected call of ListImages.
func (mr *MockVpcMockRecorder) ListImages(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListImages", reflect.TypeOf((*MockVpc)(nil).ListImages), options)
}

// ListInstances mocks base method.
func (m *MockVpc) ListInstances(options *vpcv1.ListInstancesOptions) (*vpcv1.InstanceCollection, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListInstances", options)
	ret0, _ := ret[0].(*vpcv1.InstanceCollection)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListInstances indicates an expected call of ListInstances.
func (mr *MockVpcMockRecorder) ListInstances(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListInstances", reflect.TypeOf((*MockVpc)(nil).ListInstances), options)
}

// ListKeys mocks base method.
func (m *MockVpc) ListKeys(options *vpcv1.ListKeysOptions) (*vpcv1.KeyCollection, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListKeys", options)
	ret0, _ := ret[0].(*vpcv1.KeyCollection)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListKeys indicates an expected call of ListKeys.
func (mr *MockVpcMockRecorder) ListKeys(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListKeys", reflect.TypeOf((*MockVpc)(nil).ListKeys), options)
}

// ListLoadBalancerPoolMembers mocks base method.
func (m *MockVpc) ListLoadBalancerPoolMembers(options *vpcv1.ListLoadBalancerPoolMembersOptions) (*vpcv1.LoadBalancerPoolMemberCollection, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListLoadBalancerPoolMembers", options)
	ret0, _ := ret[0].(*vpcv1.LoadBalancerPoolMemberCollection)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListLoadBalancerPoolMembers indicates an expected call of ListLoadBalancerPoolMembers.
func (mr *MockVpcMockRecorder) ListLoadBalancerPoolMembers(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListLoadBalancerPoolMembers", reflect.TypeOf((*MockVpc)(nil).ListLoadBalancerPoolMembers), options)
}

// ListLoadBalancers mocks base method.
func (m *MockVpc) ListLoadBalancers(options *vpcv1.ListLoadBalancersOptions) (*vpcv1.LoadBalancerCollection, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListLoadBalancers", options)
	ret0, _ := ret[0].(*vpcv1.LoadBalancerCollection)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListLoadBalancers indicates an expected call of ListLoadBalancers.
func (mr *MockVpcMockRecorder) ListLoadBalancers(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListLoadBalancers", reflect.TypeOf((*MockVpc)(nil).ListLoadBalancers), options)
}

// ListSecurityGroups mocks base method.
func (m *MockVpc) ListSecurityGroups(options *vpcv1.ListSecurityGroupsOptions) (*vpcv1.SecurityGroupCollection, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListSecurityGroups", options)
	ret0, _ := ret[0].(*vpcv1.SecurityGroupCollection)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListSecurityGroups indicates an expected call of ListSecurityGroups.
func (mr *MockVpcMockRecorder) ListSecurityGroups(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListSecurityGroups", reflect.TypeOf((*MockVpc)(nil).ListSecurityGroups), options)
}

// ListSubnets mocks base method.
func (m *MockVpc) ListSubnets(options *vpcv1.ListSubnetsOptions) (*vpcv1.SubnetCollection, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListSubnets", options)
	ret0, _ := ret[0].(*vpcv1.SubnetCollection)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListSubnets indicates an expected call of ListSubnets.
func (mr *MockVpcMockRecorder) ListSubnets(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListSubnets", reflect.TypeOf((*MockVpc)(nil).ListSubnets), options)
}

// ListVPCAddressPrefixes mocks base method.
func (m *MockVpc) ListVPCAddressPrefixes(options *vpcv1.ListVPCAddressPrefixesOptions) (*vpcv1.AddressPrefixCollection, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListVPCAddressPrefixes", options)
	ret0, _ := ret[0].(*vpcv1.AddressPrefixCollection)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListVPCAddressPrefixes indicates an expected call of ListVPCAddressPrefixes.
func (mr *MockVpcMockRecorder) ListVPCAddressPrefixes(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListVPCAddressPrefixes", reflect.TypeOf((*MockVpc)(nil).ListVPCAddressPrefixes), options)
}

// ListVpcs mocks base method.
func (m *MockVpc) ListVpcs(options *vpcv1.ListVpcsOptions) (*vpcv1.VPCCollection, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListVpcs", options)
	ret0, _ := ret[0].(*vpcv1.VPCCollection)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListVpcs indicates an expected call of ListVpcs.
func (mr *MockVpcMockRecorder) ListVpcs(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListVpcs", reflect.TypeOf((*MockVpc)(nil).ListVpcs), options)
}

// SetSubnetPublicGateway mocks base method.
func (m *MockVpc) SetSubnetPublicGateway(options *vpcv1.SetSubnetPublicGatewayOptions) (*vpcv1.PublicGateway, *core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetSubnetPublicGateway", options)
	ret0, _ := ret[0].(*vpcv1.PublicGateway)
	ret1, _ := ret[1].(*core.DetailedResponse)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// SetSubnetPublicGateway indicates an expected call of SetSubnetPublicGateway.
func (mr *MockVpcMockRecorder) SetSubnetPublicGateway(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSubnetPublicGateway", reflect.TypeOf((*MockVpc)(nil).SetSubnetPublicGateway), options)
}

// UnsetSubnetPublicGateway mocks base method.
func (m *MockVpc) UnsetSubnetPublicGateway(options *vpcv1.UnsetSubnetPublicGatewayOptions) (*core.DetailedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnsetSubnetPublicGateway", options)
	ret0, _ := ret[0].(*core.DetailedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UnsetSubnetPublicGateway indicates an expected call of UnsetSubnetPublicGateway.
func (mr *MockVpcMockRecorder) UnsetSubnetPublicGateway(options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnsetSubnetPublicGateway", reflect.TypeOf((*MockVpc)(nil).UnsetSubnetPublicGateway), options)
}
