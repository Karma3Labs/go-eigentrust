// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v4.25.1
// source: compute.proto

package computepb

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	trustvector "k3l.io/go-eigentrust/pkg/api/pb/trustvector"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Params struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Local trust matrix ID.
	LocalTrustId string `protobuf:"bytes,1,opt,name=local_trust_id,json=localTrustId,proto3" json:"local_trust_id,omitempty"`
	// Pre-trust vector ID.
	PreTrustId string `protobuf:"bytes,2,opt,name=pre_trust_id,json=preTrustId,proto3" json:"pre_trust_id,omitempty"`
	// Alpha value (pre-trust strength).
	Alpha *float64 `protobuf:"fixed64,3,opt,name=alpha,proto3,oneof" json:"alpha,omitempty"`
	// Epsilon value (convergence exit criteria).
	Epsilon *float64 `protobuf:"fixed64,4,opt,name=epsilon,proto3,oneof" json:"epsilon,omitempty"`
	// Global trust vector ID.
	// Must already exist, i.e. call CreateTrustVector to create one.
	// Its contents are used as the initial vector (iteration starting point).
	// If its contents are zero, e.g. a brand new trust vector is passed,
	// the pre-trust contents are copied and used as the starting point.
	// Upon return, the vector contains the computed global trust –
	// use GetTrustVector to retrieve its contents.
	GlobalTrustId string `protobuf:"bytes,5,opt,name=global_trust_id,json=globalTrustId,proto3" json:"global_trust_id,omitempty"`
	// Maximum number of iterations to perform, 0 (default): unlimited.
	MaxIterations uint32 `protobuf:"varint,6,opt,name=max_iterations,json=maxIterations,proto3" json:"max_iterations,omitempty"`
	// Where to upload the results.
	// Leave empty to disable automatic pushing.
	Destinations []*trustvector.Destination `protobuf:"bytes,7,rep,name=destinations,proto3" json:"destinations,omitempty"`
	// Positive-only trust vector ID.
	PositiveGlobalTrustId string `protobuf:"bytes,8,opt,name=positive_global_trust_id,json=positiveGlobalTrustId,proto3" json:"positive_global_trust_id,omitempty"`
}

func (x *Params) Reset() {
	*x = Params{}
	if protoimpl.UnsafeEnabled {
		mi := &file_compute_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Params) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Params) ProtoMessage() {}

func (x *Params) ProtoReflect() protoreflect.Message {
	mi := &file_compute_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Params.ProtoReflect.Descriptor instead.
func (*Params) Descriptor() ([]byte, []int) {
	return file_compute_proto_rawDescGZIP(), []int{0}
}

func (x *Params) GetLocalTrustId() string {
	if x != nil {
		return x.LocalTrustId
	}
	return ""
}

func (x *Params) GetPreTrustId() string {
	if x != nil {
		return x.PreTrustId
	}
	return ""
}

func (x *Params) GetAlpha() float64 {
	if x != nil && x.Alpha != nil {
		return *x.Alpha
	}
	return 0
}

func (x *Params) GetEpsilon() float64 {
	if x != nil && x.Epsilon != nil {
		return *x.Epsilon
	}
	return 0
}

func (x *Params) GetGlobalTrustId() string {
	if x != nil {
		return x.GlobalTrustId
	}
	return ""
}

func (x *Params) GetMaxIterations() uint32 {
	if x != nil {
		return x.MaxIterations
	}
	return 0
}

func (x *Params) GetDestinations() []*trustvector.Destination {
	if x != nil {
		return x.Destinations
	}
	return nil
}

func (x *Params) GetPositiveGlobalTrustId() string {
	if x != nil {
		return x.PositiveGlobalTrustId
	}
	return ""
}

// A periodic compute job specification.
type JobSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Compute parameters.
	// Input timestamps (such as local trust and pre-trust)
	// must have the same semantics.
	Params *Params `protobuf:"bytes,1,opt,name=params,proto3" json:"params,omitempty"`
	// Re-compute period.
	//
	// Timestamps are partitioned into time windows,
	// i.e. window number = timestamp % period.
	// A re-compute is triggered upon seeing an input update (LT or PT) whose
	// timestamp belongs to a later window than the current result timestamp.
	// The result bears the starting timestamp of the later window,
	// and reflects all the inputs BEFORE the starting timestamp.
	//
	// Example: With period=1000 and current result timestamp of 9000 (initial):
	//
	// input | window         | triggered | result timestamp
	// ==============================================
	// 9947  | [9000..10000)  | no        |
	// 10814 | [10000..11000) | yes       | 10000
	// 11438 | [11000..12000) | yes       | 11000
	// 11975 | [11000..12000) | no        |
	// 11999 | [11000..12000) | no        |
	// 12000 | [12000..13000) | yes       | 12000
	// 12014 | [12000..13000) | no        |
	//
	// (Note that the result for timestamp=12000
	// does NOT reflect the triggering input at timestamp=12000.)
	PeriodQwords []uint64 `protobuf:"varint,2,rep,packed,name=period_qwords,json=periodQwords,proto3" json:"period_qwords,omitempty"`
}

func (x *JobSpec) Reset() {
	*x = JobSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_compute_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JobSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JobSpec) ProtoMessage() {}

func (x *JobSpec) ProtoReflect() protoreflect.Message {
	mi := &file_compute_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JobSpec.ProtoReflect.Descriptor instead.
func (*JobSpec) Descriptor() ([]byte, []int) {
	return file_compute_proto_rawDescGZIP(), []int{1}
}

func (x *JobSpec) GetParams() *Params {
	if x != nil {
		return x.Params
	}
	return nil
}

func (x *JobSpec) GetPeriodQwords() []uint64 {
	if x != nil {
		return x.PeriodQwords
	}
	return nil
}

type BasicComputeRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Params *Params `protobuf:"bytes,1,opt,name=params,proto3" json:"params,omitempty"`
}

func (x *BasicComputeRequest) Reset() {
	*x = BasicComputeRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_compute_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BasicComputeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BasicComputeRequest) ProtoMessage() {}

func (x *BasicComputeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_compute_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BasicComputeRequest.ProtoReflect.Descriptor instead.
func (*BasicComputeRequest) Descriptor() ([]byte, []int) {
	return file_compute_proto_rawDescGZIP(), []int{2}
}

func (x *BasicComputeRequest) GetParams() *Params {
	if x != nil {
		return x.Params
	}
	return nil
}

type BasicComputeResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *BasicComputeResponse) Reset() {
	*x = BasicComputeResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_compute_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BasicComputeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BasicComputeResponse) ProtoMessage() {}

func (x *BasicComputeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_compute_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BasicComputeResponse.ProtoReflect.Descriptor instead.
func (*BasicComputeResponse) Descriptor() ([]byte, []int) {
	return file_compute_proto_rawDescGZIP(), []int{3}
}

type CreateJobRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Spec *JobSpec `protobuf:"bytes,1,opt,name=spec,proto3" json:"spec,omitempty"`
}

func (x *CreateJobRequest) Reset() {
	*x = CreateJobRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_compute_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateJobRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateJobRequest) ProtoMessage() {}

func (x *CreateJobRequest) ProtoReflect() protoreflect.Message {
	mi := &file_compute_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateJobRequest.ProtoReflect.Descriptor instead.
func (*CreateJobRequest) Descriptor() ([]byte, []int) {
	return file_compute_proto_rawDescGZIP(), []int{4}
}

func (x *CreateJobRequest) GetSpec() *JobSpec {
	if x != nil {
		return x.Spec
	}
	return nil
}

type CreateJobResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *CreateJobResponse) Reset() {
	*x = CreateJobResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_compute_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateJobResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateJobResponse) ProtoMessage() {}

func (x *CreateJobResponse) ProtoReflect() protoreflect.Message {
	mi := &file_compute_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateJobResponse.ProtoReflect.Descriptor instead.
func (*CreateJobResponse) Descriptor() ([]byte, []int) {
	return file_compute_proto_rawDescGZIP(), []int{5}
}

func (x *CreateJobResponse) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type DeleteJobRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *DeleteJobRequest) Reset() {
	*x = DeleteJobRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_compute_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteJobRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteJobRequest) ProtoMessage() {}

func (x *DeleteJobRequest) ProtoReflect() protoreflect.Message {
	mi := &file_compute_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteJobRequest.ProtoReflect.Descriptor instead.
func (*DeleteJobRequest) Descriptor() ([]byte, []int) {
	return file_compute_proto_rawDescGZIP(), []int{6}
}

func (x *DeleteJobRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type DeleteJobResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *DeleteJobResponse) Reset() {
	*x = DeleteJobResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_compute_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteJobResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteJobResponse) ProtoMessage() {}

func (x *DeleteJobResponse) ProtoReflect() protoreflect.Message {
	mi := &file_compute_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteJobResponse.ProtoReflect.Descriptor instead.
func (*DeleteJobResponse) Descriptor() ([]byte, []int) {
	return file_compute_proto_rawDescGZIP(), []int{7}
}

var File_compute_proto protoreflect.FileDescriptor

var file_compute_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x1a, 0x11, 0x74, 0x72, 0x75, 0x73, 0x74, 0x76,
	0x65, 0x63, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xe6, 0x02, 0x0a, 0x06,
	0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x12, 0x24, 0x0a, 0x0e, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f,
	0x74, 0x72, 0x75, 0x73, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c,
	0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x54, 0x72, 0x75, 0x73, 0x74, 0x49, 0x64, 0x12, 0x20, 0x0a, 0x0c,
	0x70, 0x72, 0x65, 0x5f, 0x74, 0x72, 0x75, 0x73, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0a, 0x70, 0x72, 0x65, 0x54, 0x72, 0x75, 0x73, 0x74, 0x49, 0x64, 0x12, 0x19,
	0x0a, 0x05, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x18, 0x03, 0x20, 0x01, 0x28, 0x01, 0x48, 0x00, 0x52,
	0x05, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x88, 0x01, 0x01, 0x12, 0x1d, 0x0a, 0x07, 0x65, 0x70, 0x73,
	0x69, 0x6c, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x01, 0x48, 0x01, 0x52, 0x07, 0x65, 0x70,
	0x73, 0x69, 0x6c, 0x6f, 0x6e, 0x88, 0x01, 0x01, 0x12, 0x26, 0x0a, 0x0f, 0x67, 0x6c, 0x6f, 0x62,
	0x61, 0x6c, 0x5f, 0x74, 0x72, 0x75, 0x73, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0d, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x54, 0x72, 0x75, 0x73, 0x74, 0x49, 0x64,
	0x12, 0x25, 0x0a, 0x0e, 0x6d, 0x61, 0x78, 0x5f, 0x69, 0x74, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0d, 0x6d, 0x61, 0x78, 0x49, 0x74, 0x65,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x3c, 0x0a, 0x0c, 0x64, 0x65, 0x73, 0x74, 0x69,
	0x6e, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x07, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x18, 0x2e,
	0x74, 0x72, 0x75, 0x73, 0x74, 0x76, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x2e, 0x44, 0x65, 0x73, 0x74,
	0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0c, 0x64, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x37, 0x0a, 0x18, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x76,
	0x65, 0x5f, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x5f, 0x74, 0x72, 0x75, 0x73, 0x74, 0x5f, 0x69,
	0x64, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x15, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x76,
	0x65, 0x47, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x54, 0x72, 0x75, 0x73, 0x74, 0x49, 0x64, 0x42, 0x08,
	0x0a, 0x06, 0x5f, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x42, 0x0a, 0x0a, 0x08, 0x5f, 0x65, 0x70, 0x73,
	0x69, 0x6c, 0x6f, 0x6e, 0x22, 0x57, 0x0a, 0x07, 0x4a, 0x6f, 0x62, 0x53, 0x70, 0x65, 0x63, 0x12,
	0x27, 0x0a, 0x06, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x0f, 0x2e, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x2e, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73,
	0x52, 0x06, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x12, 0x23, 0x0a, 0x0d, 0x70, 0x65, 0x72, 0x69,
	0x6f, 0x64, 0x5f, 0x71, 0x77, 0x6f, 0x72, 0x64, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x04, 0x52,
	0x0c, 0x70, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x51, 0x77, 0x6f, 0x72, 0x64, 0x73, 0x22, 0x3e, 0x0a,
	0x13, 0x42, 0x61, 0x73, 0x69, 0x63, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x27, 0x0a, 0x06, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x2e, 0x50,
	0x61, 0x72, 0x61, 0x6d, 0x73, 0x52, 0x06, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x22, 0x16, 0x0a,
	0x14, 0x42, 0x61, 0x73, 0x69, 0x63, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x38, 0x0a, 0x10, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4a,
	0x6f, 0x62, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x24, 0x0a, 0x04, 0x73, 0x70, 0x65,
	0x63, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74,
	0x65, 0x2e, 0x4a, 0x6f, 0x62, 0x53, 0x70, 0x65, 0x63, 0x52, 0x04, 0x73, 0x70, 0x65, 0x63, 0x22,
	0x23, 0x0a, 0x11, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4a, 0x6f, 0x62, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x02, 0x69, 0x64, 0x22, 0x22, 0x0a, 0x10, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4a, 0x6f,
	0x62, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x22, 0x13, 0x0a, 0x11, 0x44, 0x65, 0x6c, 0x65,
	0x74, 0x65, 0x4a, 0x6f, 0x62, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0xe4, 0x01,
	0x0a, 0x07, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x4d, 0x0a, 0x0c, 0x42, 0x61, 0x73,
	0x69, 0x63, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x12, 0x1c, 0x2e, 0x63, 0x6f, 0x6d, 0x70,
	0x75, 0x74, 0x65, 0x2e, 0x42, 0x61, 0x73, 0x69, 0x63, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1d, 0x2e, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74,
	0x65, 0x2e, 0x42, 0x61, 0x73, 0x69, 0x63, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x44, 0x0a, 0x09, 0x43, 0x72, 0x65, 0x61,
	0x74, 0x65, 0x4a, 0x6f, 0x62, 0x12, 0x19, 0x2e, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x2e,
	0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4a, 0x6f, 0x62, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x1a, 0x2e, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74,
	0x65, 0x4a, 0x6f, 0x62, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x44,
	0x0a, 0x09, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4a, 0x6f, 0x62, 0x12, 0x19, 0x2e, 0x63, 0x6f,
	0x6d, 0x70, 0x75, 0x74, 0x65, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4a, 0x6f, 0x62, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65,
	0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4a, 0x6f, 0x62, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x00, 0x42, 0x33, 0x5a, 0x31, 0x6b, 0x33, 0x6c, 0x2e, 0x69, 0x6f, 0x2f, 0x67,
	0x6f, 0x2d, 0x65, 0x69, 0x67, 0x65, 0x6e, 0x74, 0x72, 0x75, 0x73, 0x74, 0x2f, 0x70, 0x6b, 0x67,
	0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x62, 0x2f, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x3b,
	0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_compute_proto_rawDescOnce sync.Once
	file_compute_proto_rawDescData = file_compute_proto_rawDesc
)

func file_compute_proto_rawDescGZIP() []byte {
	file_compute_proto_rawDescOnce.Do(func() {
		file_compute_proto_rawDescData = protoimpl.X.CompressGZIP(file_compute_proto_rawDescData)
	})
	return file_compute_proto_rawDescData
}

var file_compute_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_compute_proto_goTypes = []interface{}{
	(*Params)(nil),                  // 0: compute.Params
	(*JobSpec)(nil),                 // 1: compute.JobSpec
	(*BasicComputeRequest)(nil),     // 2: compute.BasicComputeRequest
	(*BasicComputeResponse)(nil),    // 3: compute.BasicComputeResponse
	(*CreateJobRequest)(nil),        // 4: compute.CreateJobRequest
	(*CreateJobResponse)(nil),       // 5: compute.CreateJobResponse
	(*DeleteJobRequest)(nil),        // 6: compute.DeleteJobRequest
	(*DeleteJobResponse)(nil),       // 7: compute.DeleteJobResponse
	(*trustvector.Destination)(nil), // 8: trustvector.Destination
}
var file_compute_proto_depIdxs = []int32{
	8, // 0: compute.Params.destinations:type_name -> trustvector.Destination
	0, // 1: compute.JobSpec.params:type_name -> compute.Params
	0, // 2: compute.BasicComputeRequest.params:type_name -> compute.Params
	1, // 3: compute.CreateJobRequest.spec:type_name -> compute.JobSpec
	2, // 4: compute.Service.BasicCompute:input_type -> compute.BasicComputeRequest
	4, // 5: compute.Service.CreateJob:input_type -> compute.CreateJobRequest
	6, // 6: compute.Service.DeleteJob:input_type -> compute.DeleteJobRequest
	3, // 7: compute.Service.BasicCompute:output_type -> compute.BasicComputeResponse
	5, // 8: compute.Service.CreateJob:output_type -> compute.CreateJobResponse
	7, // 9: compute.Service.DeleteJob:output_type -> compute.DeleteJobResponse
	7, // [7:10] is the sub-list for method output_type
	4, // [4:7] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_compute_proto_init() }
func file_compute_proto_init() {
	if File_compute_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_compute_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Params); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_compute_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*JobSpec); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_compute_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BasicComputeRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_compute_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BasicComputeResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_compute_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateJobRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_compute_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateJobResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_compute_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteJobRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_compute_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteJobResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_compute_proto_msgTypes[0].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_compute_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_compute_proto_goTypes,
		DependencyIndexes: file_compute_proto_depIdxs,
		MessageInfos:      file_compute_proto_msgTypes,
	}.Build()
	File_compute_proto = out.File
	file_compute_proto_rawDesc = nil
	file_compute_proto_goTypes = nil
	file_compute_proto_depIdxs = nil
}
