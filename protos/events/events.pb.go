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
// 	protoc-gen-go v1.33.0
// 	protoc        v3.6.1
// source: events/events.proto

package events

import (
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

type EventSource_ValueType int32

const (
	EventSource_UNSPECIFIED EventSource_ValueType = 0
	EventSource_BOOL        EventSource_ValueType = 1
	EventSource_INT64       EventSource_ValueType = 2
	EventSource_STRING      EventSource_ValueType = 3
	EventSource_DOUBLE      EventSource_ValueType = 4
)

// Enum value maps for EventSource_ValueType.
var (
	EventSource_ValueType_name = map[int32]string{
		0: "UNSPECIFIED",
		1: "BOOL",
		2: "INT64",
		3: "STRING",
		4: "DOUBLE",
	}
	EventSource_ValueType_value = map[string]int32{
		"UNSPECIFIED": 0,
		"BOOL":        1,
		"INT64":       2,
		"STRING":      3,
		"DOUBLE":      4,
	}
)

func (x EventSource_ValueType) Enum() *EventSource_ValueType {
	p := new(EventSource_ValueType)
	*p = x
	return p
}

func (x EventSource_ValueType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (EventSource_ValueType) Descriptor() protoreflect.EnumDescriptor {
	return file_events_events_proto_enumTypes[0].Descriptor()
}

func (EventSource_ValueType) Type() protoreflect.EnumType {
	return &file_events_events_proto_enumTypes[0]
}

func (x EventSource_ValueType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use EventSource_ValueType.Descriptor instead.
func (EventSource_ValueType) EnumDescriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{1, 0}
}

type EvalNode_EvalType int32

const (
	EvalNode_UNDEFINED EvalNode_EvalType = 0
	EvalNode_EQ        EvalNode_EvalType = 1
	EvalNode_NEQ       EvalNode_EvalType = 2
	EvalNode_LT        EvalNode_EvalType = 3
	EvalNode_LTE       EvalNode_EvalType = 4
	EvalNode_GT        EvalNode_EvalType = 5
	EvalNode_GTE       EvalNode_EvalType = 6
	EvalNode_EQSTR     EvalNode_EvalType = 7
	EvalNode_SUBSTR    EvalNode_EvalType = 8
)

// Enum value maps for EvalNode_EvalType.
var (
	EvalNode_EvalType_name = map[int32]string{
		0: "UNDEFINED",
		1: "EQ",
		2: "NEQ",
		3: "LT",
		4: "LTE",
		5: "GT",
		6: "GTE",
		7: "EQSTR",
		8: "SUBSTR",
	}
	EvalNode_EvalType_value = map[string]int32{
		"UNDEFINED": 0,
		"EQ":        1,
		"NEQ":       2,
		"LT":        3,
		"LTE":       4,
		"GT":        5,
		"GTE":       6,
		"EQSTR":     7,
		"SUBSTR":    8,
	}
)

func (x EvalNode_EvalType) Enum() *EvalNode_EvalType {
	p := new(EvalNode_EvalType)
	*p = x
	return p
}

func (x EvalNode_EvalType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (EvalNode_EvalType) Descriptor() protoreflect.EnumDescriptor {
	return file_events_events_proto_enumTypes[1].Descriptor()
}

func (EvalNode_EvalType) Type() protoreflect.EnumType {
	return &file_events_events_proto_enumTypes[1]
}

func (x EvalNode_EvalType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use EvalNode_EvalType.Descriptor instead.
func (EvalNode_EvalType) EnumDescriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{3, 0}
}

type Rule struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"` // Optional - rule name
	// Required: Unique ID of the rule - must be unique across all rules.
	Id     string   `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
	Labels []string `protobuf:"bytes,3,rep,name=labels,proto3" json:"labels,omitempty"`
	// Each event will come from a single source produces a single numeric(int or
	// float) or string value.
	Source *EventSource `protobuf:"bytes,4,opt,name=source,proto3" json:"source,omitempty"`
	// Condition evaluation to decide if event should be triggered.
	Trigger *EvalNode `protobuf:"bytes,5,opt,name=trigger,proto3" json:"trigger,omitempty"`
	// We can send same event to multiple targets.
	Target       []*EventTarget `protobuf:"bytes,6,rep,name=target,proto3" json:"target,omitempty"`
	FrequencySec int64          `protobuf:"varint,7,opt,name=frequency_sec,json=frequencySec,proto3" json:"frequency_sec,omitempty"` // Event source polling frequency in seconds.
	ForceTrigger bool           `protobuf:"varint,8,opt,name=force_trigger,json=forceTrigger,proto3" json:"force_trigger,omitempty"` // Optional - for internal testing
}

func (x *Rule) Reset() {
	*x = Rule{}
	if protoimpl.UnsafeEnabled {
		mi := &file_events_events_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Rule) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Rule) ProtoMessage() {}

func (x *Rule) ProtoReflect() protoreflect.Message {
	mi := &file_events_events_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Rule.ProtoReflect.Descriptor instead.
func (*Rule) Descriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{0}
}

func (x *Rule) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Rule) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Rule) GetLabels() []string {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *Rule) GetSource() *EventSource {
	if x != nil {
		return x.Source
	}
	return nil
}

func (x *Rule) GetTrigger() *EvalNode {
	if x != nil {
		return x.Trigger
	}
	return nil
}

func (x *Rule) GetTarget() []*EventTarget {
	if x != nil {
		return x.Target
	}
	return nil
}

func (x *Rule) GetFrequencySec() int64 {
	if x != nil {
		return x.FrequencySec
	}
	return 0
}

func (x *Rule) GetForceTrigger() bool {
	if x != nil {
		return x.ForceTrigger
	}
	return false
}

type EventSource struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Source:
	//
	//	*EventSource_CloudMonitoringMetric_
	//	*EventSource_CloudLogging_
	//	*EventSource_Metadata_
	//	*EventSource_GuestLog_
	Source isEventSource_Source `protobuf_oneof:"source"`
}

func (x *EventSource) Reset() {
	*x = EventSource{}
	if protoimpl.UnsafeEnabled {
		mi := &file_events_events_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventSource) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventSource) ProtoMessage() {}

func (x *EventSource) ProtoReflect() protoreflect.Message {
	mi := &file_events_events_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventSource.ProtoReflect.Descriptor instead.
func (*EventSource) Descriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{1}
}

func (m *EventSource) GetSource() isEventSource_Source {
	if m != nil {
		return m.Source
	}
	return nil
}

func (x *EventSource) GetCloudMonitoringMetric() *EventSource_CloudMonitoringMetric {
	if x, ok := x.GetSource().(*EventSource_CloudMonitoringMetric_); ok {
		return x.CloudMonitoringMetric
	}
	return nil
}

func (x *EventSource) GetCloudLogging() *EventSource_CloudLogging {
	if x, ok := x.GetSource().(*EventSource_CloudLogging_); ok {
		return x.CloudLogging
	}
	return nil
}

func (x *EventSource) GetMetadata() *EventSource_Metadata {
	if x, ok := x.GetSource().(*EventSource_Metadata_); ok {
		return x.Metadata
	}
	return nil
}

func (x *EventSource) GetGuestLog() *EventSource_GuestLog {
	if x, ok := x.GetSource().(*EventSource_GuestLog_); ok {
		return x.GuestLog
	}
	return nil
}

type isEventSource_Source interface {
	isEventSource_Source()
}

type EventSource_CloudMonitoringMetric_ struct {
	CloudMonitoringMetric *EventSource_CloudMonitoringMetric `protobuf:"bytes,1,opt,name=cloud_monitoring_metric,json=cloudMonitoringMetric,proto3,oneof"`
}

type EventSource_CloudLogging_ struct {
	CloudLogging *EventSource_CloudLogging `protobuf:"bytes,2,opt,name=cloud_logging,json=cloudLogging,proto3,oneof"`
}

type EventSource_Metadata_ struct {
	Metadata *EventSource_Metadata `protobuf:"bytes,3,opt,name=metadata,proto3,oneof"`
}

type EventSource_GuestLog_ struct {
	GuestLog *EventSource_GuestLog `protobuf:"bytes,4,opt,name=guest_log,json=guestLog,proto3,oneof"`
}

func (*EventSource_CloudMonitoringMetric_) isEventSource_Source() {}

func (*EventSource_CloudLogging_) isEventSource_Source() {}

func (*EventSource_Metadata_) isEventSource_Source() {}

func (*EventSource_GuestLog_) isEventSource_Source() {}

type EventTarget struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Target:
	//
	//	*EventTarget_HttpEndpoint
	//	*EventTarget_FileEndpoint
	Target isEventTarget_Target `protobuf_oneof:"target"`
}

func (x *EventTarget) Reset() {
	*x = EventTarget{}
	if protoimpl.UnsafeEnabled {
		mi := &file_events_events_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventTarget) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventTarget) ProtoMessage() {}

func (x *EventTarget) ProtoReflect() protoreflect.Message {
	mi := &file_events_events_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventTarget.ProtoReflect.Descriptor instead.
func (*EventTarget) Descriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{2}
}

func (m *EventTarget) GetTarget() isEventTarget_Target {
	if m != nil {
		return m.Target
	}
	return nil
}

func (x *EventTarget) GetHttpEndpoint() string {
	if x, ok := x.GetTarget().(*EventTarget_HttpEndpoint); ok {
		return x.HttpEndpoint
	}
	return ""
}

func (x *EventTarget) GetFileEndpoint() string {
	if x, ok := x.GetTarget().(*EventTarget_FileEndpoint); ok {
		return x.FileEndpoint
	}
	return ""
}

type isEventTarget_Target interface {
	isEventTarget_Target()
}

type EventTarget_HttpEndpoint struct {
	HttpEndpoint string `protobuf:"bytes,1,opt,name=http_endpoint,json=httpEndpoint,proto3,oneof"`
}

type EventTarget_FileEndpoint struct {
	FileEndpoint string `protobuf:"bytes,2,opt,name=file_endpoint,json=fileEndpoint,proto3,oneof"`
}

func (*EventTarget_HttpEndpoint) isEventTarget_Target() {}

func (*EventTarget_FileEndpoint) isEventTarget_Target() {}

type EvalNode struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Rhs       string            `protobuf:"bytes,2,opt,name=rhs,proto3" json:"rhs,omitempty"`
	Operation EvalNode_EvalType `protobuf:"varint,3,opt,name=operation,proto3,enum=sapagent.protos.events.EvalNode_EvalType" json:"operation,omitempty"`
}

func (x *EvalNode) Reset() {
	*x = EvalNode{}
	if protoimpl.UnsafeEnabled {
		mi := &file_events_events_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EvalNode) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EvalNode) ProtoMessage() {}

func (x *EvalNode) ProtoReflect() protoreflect.Message {
	mi := &file_events_events_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EvalNode.ProtoReflect.Descriptor instead.
func (*EvalNode) Descriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{3}
}

func (x *EvalNode) GetRhs() string {
	if x != nil {
		return x.Rhs
	}
	return ""
}

func (x *EvalNode) GetOperation() EvalNode_EvalType {
	if x != nil {
		return x.Operation
	}
	return EvalNode_UNDEFINED
}

type EventSource_CloudMonitoringMetric struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	MetricUrl string `protobuf:"bytes,1,opt,name=metric_url,json=metricUrl,proto3" json:"metric_url,omitempty"` // ex: workload.googleapis.com/sap/hana/myevent
	// User specifies either the label name or the type of the metric value.
	//
	// Types that are assignable to Metric:
	//
	//	*EventSource_CloudMonitoringMetric_LabelName
	//	*EventSource_CloudMonitoringMetric_MetricValueType
	Metric isEventSource_CloudMonitoringMetric_Metric `protobuf_oneof:"metric"`
}

func (x *EventSource_CloudMonitoringMetric) Reset() {
	*x = EventSource_CloudMonitoringMetric{}
	if protoimpl.UnsafeEnabled {
		mi := &file_events_events_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventSource_CloudMonitoringMetric) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventSource_CloudMonitoringMetric) ProtoMessage() {}

func (x *EventSource_CloudMonitoringMetric) ProtoReflect() protoreflect.Message {
	mi := &file_events_events_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventSource_CloudMonitoringMetric.ProtoReflect.Descriptor instead.
func (*EventSource_CloudMonitoringMetric) Descriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{1, 0}
}

func (x *EventSource_CloudMonitoringMetric) GetMetricUrl() string {
	if x != nil {
		return x.MetricUrl
	}
	return ""
}

func (m *EventSource_CloudMonitoringMetric) GetMetric() isEventSource_CloudMonitoringMetric_Metric {
	if m != nil {
		return m.Metric
	}
	return nil
}

func (x *EventSource_CloudMonitoringMetric) GetLabelName() string {
	if x, ok := x.GetMetric().(*EventSource_CloudMonitoringMetric_LabelName); ok {
		return x.LabelName
	}
	return ""
}

func (x *EventSource_CloudMonitoringMetric) GetMetricValueType() EventSource_ValueType {
	if x, ok := x.GetMetric().(*EventSource_CloudMonitoringMetric_MetricValueType); ok {
		return x.MetricValueType
	}
	return EventSource_UNSPECIFIED
}

type isEventSource_CloudMonitoringMetric_Metric interface {
	isEventSource_CloudMonitoringMetric_Metric()
}

type EventSource_CloudMonitoringMetric_LabelName struct {
	LabelName string `protobuf:"bytes,2,opt,name=label_name,json=labelName,proto3,oneof"`
}

type EventSource_CloudMonitoringMetric_MetricValueType struct {
	// We are using the metric value, specify the value type.
	MetricValueType EventSource_ValueType `protobuf:"varint,3,opt,name=metric_value_type,json=metricValueType,proto3,enum=sapagent.protos.events.EventSource_ValueType,oneof"`
}

func (*EventSource_CloudMonitoringMetric_LabelName) isEventSource_CloudMonitoringMetric_Metric() {}

func (*EventSource_CloudMonitoringMetric_MetricValueType) isEventSource_CloudMonitoringMetric_Metric() {
}

type EventSource_CloudLogging struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Logging query written in
	// https://cloud.google.com/logging/docs/view/logging-query-language
	LogQuery string `protobuf:"bytes,1,opt,name=log_query,json=logQuery,proto3" json:"log_query,omitempty"`
	// Value type returned by the cloud logging query.
	ValueType EventSource_ValueType `protobuf:"varint,2,opt,name=value_type,json=valueType,proto3,enum=sapagent.protos.events.EventSource_ValueType" json:"value_type,omitempty"`
}

func (x *EventSource_CloudLogging) Reset() {
	*x = EventSource_CloudLogging{}
	if protoimpl.UnsafeEnabled {
		mi := &file_events_events_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventSource_CloudLogging) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventSource_CloudLogging) ProtoMessage() {}

func (x *EventSource_CloudLogging) ProtoReflect() protoreflect.Message {
	mi := &file_events_events_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventSource_CloudLogging.ProtoReflect.Descriptor instead.
func (*EventSource_CloudLogging) Descriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{1, 1}
}

func (x *EventSource_CloudLogging) GetLogQuery() string {
	if x != nil {
		return x.LogQuery
	}
	return ""
}

func (x *EventSource_CloudLogging) GetValueType() EventSource_ValueType {
	if x != nil {
		return x.ValueType
	}
	return EventSource_UNSPECIFIED
}

type EventSource_Metadata struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Url       string                `protobuf:"bytes,1,opt,name=url,proto3" json:"url,omitempty"`
	ValueType EventSource_ValueType `protobuf:"varint,2,opt,name=value_type,json=valueType,proto3,enum=sapagent.protos.events.EventSource_ValueType" json:"value_type,omitempty"` // Value type returned by the GCP METADATA.
}

func (x *EventSource_Metadata) Reset() {
	*x = EventSource_Metadata{}
	if protoimpl.UnsafeEnabled {
		mi := &file_events_events_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventSource_Metadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventSource_Metadata) ProtoMessage() {}

func (x *EventSource_Metadata) ProtoReflect() protoreflect.Message {
	mi := &file_events_events_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventSource_Metadata.ProtoReflect.Descriptor instead.
func (*EventSource_Metadata) Descriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{1, 2}
}

func (x *EventSource_Metadata) GetUrl() string {
	if x != nil {
		return x.Url
	}
	return ""
}

func (x *EventSource_Metadata) GetValueType() EventSource_ValueType {
	if x != nil {
		return x.ValueType
	}
	return EventSource_UNSPECIFIED
}

type EventSource_GuestLog struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// A Linux command to search for particular string in a file, ex:
	// grep "ERROR" /var/log/google-cloud-sap-agent.log
	Command   string                `protobuf:"bytes,1,opt,name=command,proto3" json:"command,omitempty"`
	ValueType EventSource_ValueType `protobuf:"varint,2,opt,name=value_type,json=valueType,proto3,enum=sapagent.protos.events.EventSource_ValueType" json:"value_type,omitempty"` // Value type returned by the command.
}

func (x *EventSource_GuestLog) Reset() {
	*x = EventSource_GuestLog{}
	if protoimpl.UnsafeEnabled {
		mi := &file_events_events_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventSource_GuestLog) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventSource_GuestLog) ProtoMessage() {}

func (x *EventSource_GuestLog) ProtoReflect() protoreflect.Message {
	mi := &file_events_events_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventSource_GuestLog.ProtoReflect.Descriptor instead.
func (*EventSource_GuestLog) Descriptor() ([]byte, []int) {
	return file_events_events_proto_rawDescGZIP(), []int{1, 3}
}

func (x *EventSource_GuestLog) GetCommand() string {
	if x != nil {
		return x.Command
	}
	return ""
}

func (x *EventSource_GuestLog) GetValueType() EventSource_ValueType {
	if x != nil {
		return x.ValueType
	}
	return EventSource_UNSPECIFIED
}

var File_events_events_proto protoreflect.FileDescriptor

var file_events_events_proto_rawDesc = []byte{
	0x0a, 0x13, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x16, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x22, 0xc2, 0x02,
	0x0a, 0x04, 0x52, 0x75, 0x6c, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x6c, 0x61,
	0x62, 0x65, 0x6c, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x6c, 0x61, 0x62, 0x65,
	0x6c, 0x73, 0x12, 0x3b, 0x0a, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x23, 0x2e, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e,
	0x74, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x52, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x12,
	0x3a, 0x0a, 0x07, 0x74, 0x72, 0x69, 0x67, 0x67, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x20, 0x2e, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x61, 0x6c, 0x4e, 0x6f,
	0x64, 0x65, 0x52, 0x07, 0x74, 0x72, 0x69, 0x67, 0x67, 0x65, 0x72, 0x12, 0x3b, 0x0a, 0x06, 0x74,
	0x61, 0x72, 0x67, 0x65, 0x74, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x23, 0x2e, 0x73, 0x61,
	0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x65, 0x76,
	0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x54, 0x61, 0x72, 0x67, 0x65, 0x74,
	0x52, 0x06, 0x74, 0x61, 0x72, 0x67, 0x65, 0x74, 0x12, 0x23, 0x0a, 0x0d, 0x66, 0x72, 0x65, 0x71,
	0x75, 0x65, 0x6e, 0x63, 0x79, 0x5f, 0x73, 0x65, 0x63, 0x18, 0x07, 0x20, 0x01, 0x28, 0x03, 0x52,
	0x0c, 0x66, 0x72, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x79, 0x53, 0x65, 0x63, 0x12, 0x23, 0x0a,
	0x0d, 0x66, 0x6f, 0x72, 0x63, 0x65, 0x5f, 0x74, 0x72, 0x69, 0x67, 0x67, 0x65, 0x72, 0x18, 0x08,
	0x20, 0x01, 0x28, 0x08, 0x52, 0x0c, 0x66, 0x6f, 0x72, 0x63, 0x65, 0x54, 0x72, 0x69, 0x67, 0x67,
	0x65, 0x72, 0x22, 0xe5, 0x07, 0x0a, 0x0b, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x53, 0x6f, 0x75, 0x72,
	0x63, 0x65, 0x12, 0x73, 0x0a, 0x17, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x5f, 0x6d, 0x6f, 0x6e, 0x69,
	0x74, 0x6f, 0x72, 0x69, 0x6e, 0x67, 0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x39, 0x2e, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x65,
	0x6e, 0x74, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x4d, 0x6f,
	0x6e, 0x69, 0x74, 0x6f, 0x72, 0x69, 0x6e, 0x67, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x48, 0x00,
	0x52, 0x15, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x4d, 0x6f, 0x6e, 0x69, 0x74, 0x6f, 0x72, 0x69, 0x6e,
	0x67, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x12, 0x57, 0x0a, 0x0d, 0x63, 0x6c, 0x6f, 0x75, 0x64,
	0x5f, 0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x30,
	0x2e, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73,
	0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x53, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x2e, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x4c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67,
	0x48, 0x00, 0x52, 0x0c, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x4c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67,
	0x12, 0x4a, 0x0a, 0x08, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e,
	0x74, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61,
	0x48, 0x00, 0x52, 0x08, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x4b, 0x0a, 0x09,
	0x67, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x6c, 0x6f, 0x67, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x2c, 0x2e, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x53, 0x6f,
	0x75, 0x72, 0x63, 0x65, 0x2e, 0x47, 0x75, 0x65, 0x73, 0x74, 0x4c, 0x6f, 0x67, 0x48, 0x00, 0x52,
	0x08, 0x67, 0x75, 0x65, 0x73, 0x74, 0x4c, 0x6f, 0x67, 0x1a, 0xbe, 0x01, 0x0a, 0x15, 0x43, 0x6c,
	0x6f, 0x75, 0x64, 0x4d, 0x6f, 0x6e, 0x69, 0x74, 0x6f, 0x72, 0x69, 0x6e, 0x67, 0x4d, 0x65, 0x74,
	0x72, 0x69, 0x63, 0x12, 0x1d, 0x0a, 0x0a, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x5f, 0x75, 0x72,
	0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x55,
	0x72, 0x6c, 0x12, 0x1f, 0x0a, 0x0a, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x5f, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x09, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x4e,
	0x61, 0x6d, 0x65, 0x12, 0x5b, 0x0a, 0x11, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x5f, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x2d,
	0x2e, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73,
	0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x53, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x54, 0x79, 0x70, 0x65, 0x48, 0x00, 0x52,
	0x0f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x54, 0x79, 0x70, 0x65,
	0x42, 0x08, 0x0a, 0x06, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x1a, 0x79, 0x0a, 0x0c, 0x43, 0x6c,
	0x6f, 0x75, 0x64, 0x4c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x12, 0x1b, 0x0a, 0x09, 0x6c, 0x6f,
	0x67, 0x5f, 0x71, 0x75, 0x65, 0x72, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6c,
	0x6f, 0x67, 0x51, 0x75, 0x65, 0x72, 0x79, 0x12, 0x4c, 0x0a, 0x0a, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x2d, 0x2e, 0x73, 0x61,
	0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x65, 0x76,
	0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x54, 0x79, 0x70, 0x65, 0x52, 0x09, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x54, 0x79, 0x70, 0x65, 0x1a, 0x6a, 0x0a, 0x08, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74,
	0x61, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03,
	0x75, 0x72, 0x6c, 0x12, 0x4c, 0x0a, 0x0a, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x5f, 0x74, 0x79, 0x70,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x2d, 0x2e, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65,
	0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73,
	0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x54, 0x79, 0x70, 0x65, 0x52, 0x09, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x54, 0x79, 0x70,
	0x65, 0x1a, 0x72, 0x0a, 0x08, 0x47, 0x75, 0x65, 0x73, 0x74, 0x4c, 0x6f, 0x67, 0x12, 0x18, 0x0a,
	0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x12, 0x4c, 0x0a, 0x0a, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x2d, 0x2e, 0x73, 0x61,
	0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x65, 0x76,
	0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x54, 0x79, 0x70, 0x65, 0x52, 0x09, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x54, 0x79, 0x70, 0x65, 0x22, 0x49, 0x0a, 0x09, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x0f, 0x0a, 0x0b, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45,
	0x44, 0x10, 0x00, 0x12, 0x08, 0x0a, 0x04, 0x42, 0x4f, 0x4f, 0x4c, 0x10, 0x01, 0x12, 0x09, 0x0a,
	0x05, 0x49, 0x4e, 0x54, 0x36, 0x34, 0x10, 0x02, 0x12, 0x0a, 0x0a, 0x06, 0x53, 0x54, 0x52, 0x49,
	0x4e, 0x47, 0x10, 0x03, 0x12, 0x0a, 0x0a, 0x06, 0x44, 0x4f, 0x55, 0x42, 0x4c, 0x45, 0x10, 0x04,
	0x42, 0x08, 0x0a, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x22, 0x65, 0x0a, 0x0b, 0x45, 0x76,
	0x65, 0x6e, 0x74, 0x54, 0x61, 0x72, 0x67, 0x65, 0x74, 0x12, 0x25, 0x0a, 0x0d, 0x68, 0x74, 0x74,
	0x70, 0x5f, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x48, 0x00, 0x52, 0x0c, 0x68, 0x74, 0x74, 0x70, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74,
	0x12, 0x25, 0x0a, 0x0d, 0x66, 0x69, 0x6c, 0x65, 0x5f, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x0c, 0x66, 0x69, 0x6c, 0x65, 0x45,
	0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x42, 0x08, 0x0a, 0x06, 0x74, 0x61, 0x72, 0x67, 0x65,
	0x74, 0x22, 0xca, 0x01, 0x0a, 0x08, 0x45, 0x76, 0x61, 0x6c, 0x4e, 0x6f, 0x64, 0x65, 0x12, 0x10,
	0x0a, 0x03, 0x72, 0x68, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x72, 0x68, 0x73,
	0x12, 0x47, 0x0a, 0x09, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x29, 0x2e, 0x73, 0x61, 0x70, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x2e, 0x45, 0x76, 0x61,
	0x6c, 0x4e, 0x6f, 0x64, 0x65, 0x2e, 0x45, 0x76, 0x61, 0x6c, 0x54, 0x79, 0x70, 0x65, 0x52, 0x09,
	0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x63, 0x0a, 0x08, 0x45, 0x76, 0x61,
	0x6c, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0d, 0x0a, 0x09, 0x55, 0x4e, 0x44, 0x45, 0x46, 0x49, 0x4e,
	0x45, 0x44, 0x10, 0x00, 0x12, 0x06, 0x0a, 0x02, 0x45, 0x51, 0x10, 0x01, 0x12, 0x07, 0x0a, 0x03,
	0x4e, 0x45, 0x51, 0x10, 0x02, 0x12, 0x06, 0x0a, 0x02, 0x4c, 0x54, 0x10, 0x03, 0x12, 0x07, 0x0a,
	0x03, 0x4c, 0x54, 0x45, 0x10, 0x04, 0x12, 0x06, 0x0a, 0x02, 0x47, 0x54, 0x10, 0x05, 0x12, 0x07,
	0x0a, 0x03, 0x47, 0x54, 0x45, 0x10, 0x06, 0x12, 0x09, 0x0a, 0x05, 0x45, 0x51, 0x53, 0x54, 0x52,
	0x10, 0x07, 0x12, 0x0a, 0x0a, 0x06, 0x53, 0x55, 0x42, 0x53, 0x54, 0x52, 0x10, 0x08, 0x42, 0x02,
	0x50, 0x01, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_events_events_proto_rawDescOnce sync.Once
	file_events_events_proto_rawDescData = file_events_events_proto_rawDesc
)

func file_events_events_proto_rawDescGZIP() []byte {
	file_events_events_proto_rawDescOnce.Do(func() {
		file_events_events_proto_rawDescData = protoimpl.X.CompressGZIP(file_events_events_proto_rawDescData)
	})
	return file_events_events_proto_rawDescData
}

var file_events_events_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_events_events_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_events_events_proto_goTypes = []interface{}{
	(EventSource_ValueType)(0),                // 0: sapagent.protos.events.EventSource.ValueType
	(EvalNode_EvalType)(0),                    // 1: sapagent.protos.events.EvalNode.EvalType
	(*Rule)(nil),                              // 2: sapagent.protos.events.Rule
	(*EventSource)(nil),                       // 3: sapagent.protos.events.EventSource
	(*EventTarget)(nil),                       // 4: sapagent.protos.events.EventTarget
	(*EvalNode)(nil),                          // 5: sapagent.protos.events.EvalNode
	(*EventSource_CloudMonitoringMetric)(nil), // 6: sapagent.protos.events.EventSource.CloudMonitoringMetric
	(*EventSource_CloudLogging)(nil),          // 7: sapagent.protos.events.EventSource.CloudLogging
	(*EventSource_Metadata)(nil),              // 8: sapagent.protos.events.EventSource.Metadata
	(*EventSource_GuestLog)(nil),              // 9: sapagent.protos.events.EventSource.GuestLog
}
var file_events_events_proto_depIdxs = []int32{
	3,  // 0: sapagent.protos.events.Rule.source:type_name -> sapagent.protos.events.EventSource
	5,  // 1: sapagent.protos.events.Rule.trigger:type_name -> sapagent.protos.events.EvalNode
	4,  // 2: sapagent.protos.events.Rule.target:type_name -> sapagent.protos.events.EventTarget
	6,  // 3: sapagent.protos.events.EventSource.cloud_monitoring_metric:type_name -> sapagent.protos.events.EventSource.CloudMonitoringMetric
	7,  // 4: sapagent.protos.events.EventSource.cloud_logging:type_name -> sapagent.protos.events.EventSource.CloudLogging
	8,  // 5: sapagent.protos.events.EventSource.metadata:type_name -> sapagent.protos.events.EventSource.Metadata
	9,  // 6: sapagent.protos.events.EventSource.guest_log:type_name -> sapagent.protos.events.EventSource.GuestLog
	1,  // 7: sapagent.protos.events.EvalNode.operation:type_name -> sapagent.protos.events.EvalNode.EvalType
	0,  // 8: sapagent.protos.events.EventSource.CloudMonitoringMetric.metric_value_type:type_name -> sapagent.protos.events.EventSource.ValueType
	0,  // 9: sapagent.protos.events.EventSource.CloudLogging.value_type:type_name -> sapagent.protos.events.EventSource.ValueType
	0,  // 10: sapagent.protos.events.EventSource.Metadata.value_type:type_name -> sapagent.protos.events.EventSource.ValueType
	0,  // 11: sapagent.protos.events.EventSource.GuestLog.value_type:type_name -> sapagent.protos.events.EventSource.ValueType
	12, // [12:12] is the sub-list for method output_type
	12, // [12:12] is the sub-list for method input_type
	12, // [12:12] is the sub-list for extension type_name
	12, // [12:12] is the sub-list for extension extendee
	0,  // [0:12] is the sub-list for field type_name
}

func init() { file_events_events_proto_init() }
func file_events_events_proto_init() {
	if File_events_events_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_events_events_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Rule); i {
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
		file_events_events_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventSource); i {
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
		file_events_events_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventTarget); i {
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
		file_events_events_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EvalNode); i {
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
		file_events_events_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventSource_CloudMonitoringMetric); i {
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
		file_events_events_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventSource_CloudLogging); i {
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
		file_events_events_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventSource_Metadata); i {
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
		file_events_events_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventSource_GuestLog); i {
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
	file_events_events_proto_msgTypes[1].OneofWrappers = []interface{}{
		(*EventSource_CloudMonitoringMetric_)(nil),
		(*EventSource_CloudLogging_)(nil),
		(*EventSource_Metadata_)(nil),
		(*EventSource_GuestLog_)(nil),
	}
	file_events_events_proto_msgTypes[2].OneofWrappers = []interface{}{
		(*EventTarget_HttpEndpoint)(nil),
		(*EventTarget_FileEndpoint)(nil),
	}
	file_events_events_proto_msgTypes[4].OneofWrappers = []interface{}{
		(*EventSource_CloudMonitoringMetric_LabelName)(nil),
		(*EventSource_CloudMonitoringMetric_MetricValueType)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_events_events_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_events_events_proto_goTypes,
		DependencyIndexes: file_events_events_proto_depIdxs,
		EnumInfos:         file_events_events_proto_enumTypes,
		MessageInfos:      file_events_events_proto_msgTypes,
	}.Build()
	File_events_events_proto = out.File
	file_events_events_proto_rawDesc = nil
	file_events_events_proto_goTypes = nil
	file_events_events_proto_depIdxs = nil
}
