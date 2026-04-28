package eventlog

import (
	"sort"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

var pebbleStoredEventDescriptor = buildPebbleStoredEventDescriptor()

func marshalPebbleStoredEvent(event *eventlog.Event) ([]byte, error) {
	message := dynamicpb.NewMessage(pebbleStoredEventDescriptor)
	fields := pebbleStoredEventDescriptor.Fields()

	message.Set(fields.ByName("topic"), protoreflect.ValueOfString(event.Topic))
	message.Set(fields.ByName("offset"), protoreflect.ValueOfInt64(event.Offset))
	message.Set(fields.ByName("payload"), protoreflect.ValueOfBytes(append([]byte(nil), event.Payload...)))
	message.Set(fields.ByName("timestamp_nano"), protoreflect.ValueOfInt64(event.Timestamp.UTC().UnixNano()))

	headersField := fields.ByName("headers")
	headers := message.Mutable(headersField).List()
	headerDescriptor := headersField.Message()

	headerKeys := make([]string, 0, len(event.Headers))
	for key := range event.Headers {
		headerKeys = append(headerKeys, key)
	}
	sort.Strings(headerKeys)

	for _, key := range headerKeys {
		header := dynamicpb.NewMessage(headerDescriptor)
		headerFields := headerDescriptor.Fields()
		header.Set(headerFields.ByName("key"), protoreflect.ValueOfString(key))
		header.Set(headerFields.ByName("value"), protoreflect.ValueOfString(event.Headers[key]))
		headers.Append(protoreflect.ValueOfMessage(header))
	}

	return proto.MarshalOptions{Deterministic: true}.Marshal(message)
}

func unmarshalPebbleStoredEvent(encoded []byte) (*eventlog.Event, error) {
	message := dynamicpb.NewMessage(pebbleStoredEventDescriptor)
	if err := proto.Unmarshal(encoded, message); err != nil {
		return nil, err
	}

	fields := pebbleStoredEventDescriptor.Fields()
	headers := make(map[string]string)
	headerValues := message.Get(fields.ByName("headers")).List()
	headerDescriptor := fields.ByName("headers").Message()
	keyField := headerDescriptor.Fields().ByName("key")
	valueField := headerDescriptor.Fields().ByName("value")

	for i := 0; i < headerValues.Len(); i++ {
		header := headerValues.Get(i).Message()
		headers[header.Get(keyField).String()] = header.Get(valueField).String()
	}

	return &eventlog.Event{
		Topic:     message.Get(fields.ByName("topic")).String(),
		Offset:    message.Get(fields.ByName("offset")).Int(),
		Payload:   append([]byte(nil), message.Get(fields.ByName("payload")).Bytes()...),
		Headers:   headers,
		Timestamp: time.Unix(0, message.Get(fields.ByName("timestamp_nano")).Int()).UTC(),
	}, nil
}

func buildPebbleStoredEventDescriptor() protoreflect.MessageDescriptor {
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Syntax:  stringPtr("proto3"),
		Name:    stringPtr("eventlog/storage.proto"),
		Package: stringPtr("eventmesh.eventlog.storage.v1"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPtr("StoredEvent"),
				Field: []*descriptorpb.FieldDescriptorProto{
					protoField("topic", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
					protoField("offset", 2, descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
					protoField("payload", 3, descriptorpb.FieldDescriptorProto_TYPE_BYTES, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
					protoField("headers", 4, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".eventmesh.eventlog.storage.v1.StoredEvent.Header"),
					protoField("timestamp_nano", 5, descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
				},
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name: stringPtr("Header"),
						Field: []*descriptorpb.FieldDescriptorProto{
							protoField("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
							protoField("value", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
						},
					},
				},
			},
		},
	}, nil)
	if err != nil {
		panic(err)
	}

	return file.Messages().ByName("StoredEvent")
}

func protoField(name string, number int32, fieldType descriptorpb.FieldDescriptorProto_Type, label descriptorpb.FieldDescriptorProto_Label, typeName string) *descriptorpb.FieldDescriptorProto {
	field := &descriptorpb.FieldDescriptorProto{
		Name:   stringPtr(name),
		Number: int32Ptr(number),
		Label:  labelPtr(label),
		Type:   typePtr(fieldType),
	}
	if typeName != "" {
		field.TypeName = stringPtr(typeName)
	}
	return field
}

func stringPtr(value string) *string {
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}

func typePtr(value descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &value
}

func labelPtr(value descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label {
	return &value
}
