//
//Copyright 2023 Google LLC
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//https://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v3.6.1
// source: collectiondefinition/collectiondefinition.proto

package collectiondefinition

import (
	wlmvalidation "github.com/GoogleCloudPlatform/sapagent/protos/wlmvalidation"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type CollectionDefinition struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	WorkloadValidation *wlmvalidation.WorkloadValidation `protobuf:"bytes,1,opt,name=workload_validation,json=workloadValidation,proto3" json:"workload_validation,omitempty"`
}

func (x *CollectionDefinition) Reset() {
	*x = CollectionDefinition{}
	if protoimpl.UnsafeEnabled {
		mi := &file_collectiondefinition_collectiondefinition_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CollectionDefinition) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CollectionDefinition) ProtoMessage() {}

func (x *CollectionDefinition) ProtoReflect() protoreflect.Message {
	mi := &file_collectiondefinition_collectiondefinition_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CollectionDefinition.ProtoReflect.Descriptor instead.
func (*CollectionDefinition) Descriptor() ([]byte, []int) {
	return file_collectiondefinition_collectiondefinition_proto_rawDescGZIP(), []int{0}
}

func (x *CollectionDefinition) GetWorkloadValidation() *wlmvalidation.WorkloadValidation {
	if x != nil {
		return x.WorkloadValidation
	}
	return nil
}

var File_collectiondefinition_collectiondefinition_proto protoreflect.FileDescriptor

var file_collectiondefinition_collectiondefinition_proto_rawDesc = []byte{
	0x0a, 0x2f, 0x63, 0x6f, 0x6c, 0x6c, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x64, 0x65, 0x66, 0x69,
	0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x63, 0x6f, 0x6c, 0x6c, 0x65, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x36, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2e, 0x70, 0x61, 0x72, 0x74, 0x6e, 0x65, 0x72,
	0x73, 0x2e, 0x73, 0x61, 0x70, 0x2e, 0x67, 0x63, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x63, 0x6f, 0x6c, 0x6c, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x64,
	0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x1a, 0x21, 0x77, 0x6c, 0x6d, 0x76, 0x61,
	0x6c, 0x69, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x77, 0x6c, 0x6d, 0x76, 0x61, 0x6c, 0x69,
	0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x8c, 0x01, 0x0a,
	0x14, 0x43, 0x6f, 0x6c, 0x6c, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x44, 0x65, 0x66, 0x69, 0x6e,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x74, 0x0a, 0x13, 0x77, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61,
	0x64, 0x5f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x43, 0x2e, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2e, 0x70, 0x61, 0x72, 0x74, 0x6e,
	0x65, 0x72, 0x73, 0x2e, 0x73, 0x61, 0x70, 0x2e, 0x67, 0x63, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x77, 0x6c, 0x6d, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x57, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x56, 0x61, 0x6c,
	0x69, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x12, 0x77, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61,
	0x64, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_collectiondefinition_collectiondefinition_proto_rawDescOnce sync.Once
	file_collectiondefinition_collectiondefinition_proto_rawDescData = file_collectiondefinition_collectiondefinition_proto_rawDesc
)

func file_collectiondefinition_collectiondefinition_proto_rawDescGZIP() []byte {
	file_collectiondefinition_collectiondefinition_proto_rawDescOnce.Do(func() {
		file_collectiondefinition_collectiondefinition_proto_rawDescData = protoimpl.X.CompressGZIP(file_collectiondefinition_collectiondefinition_proto_rawDescData)
	})
	return file_collectiondefinition_collectiondefinition_proto_rawDescData
}

var file_collectiondefinition_collectiondefinition_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_collectiondefinition_collectiondefinition_proto_goTypes = []interface{}{
	(*CollectionDefinition)(nil),             // 0: cloud.partners.sap.gcagent.protos.collectiondefinition.CollectionDefinition
	(*wlmvalidation.WorkloadValidation)(nil), // 1: cloud.partners.sap.gcagent.protos.wlmvalidation.WorkloadValidation
}
var file_collectiondefinition_collectiondefinition_proto_depIdxs = []int32{
	1, // 0: cloud.partners.sap.gcagent.protos.collectiondefinition.CollectionDefinition.workload_validation:type_name -> cloud.partners.sap.gcagent.protos.wlmvalidation.WorkloadValidation
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_collectiondefinition_collectiondefinition_proto_init() }
func file_collectiondefinition_collectiondefinition_proto_init() {
	if File_collectiondefinition_collectiondefinition_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_collectiondefinition_collectiondefinition_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CollectionDefinition); i {
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_collectiondefinition_collectiondefinition_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_collectiondefinition_collectiondefinition_proto_goTypes,
		DependencyIndexes: file_collectiondefinition_collectiondefinition_proto_depIdxs,
		MessageInfos:      file_collectiondefinition_collectiondefinition_proto_msgTypes,
	}.Build()
	File_collectiondefinition_collectiondefinition_proto = out.File
	file_collectiondefinition_collectiondefinition_proto_rawDesc = nil
	file_collectiondefinition_collectiondefinition_proto_goTypes = nil
	file_collectiondefinition_collectiondefinition_proto_depIdxs = nil
}
