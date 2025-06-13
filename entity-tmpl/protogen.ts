import * as t from 'proto-parser';
// https://github.com/lancewuz/proto-parser
// npx tsx ./protogen.ts

const content = `
syntax = 'proto3';
import "proto-template/template.proto";
message Foo {
  option deprecated = true;
  option message_set_wire_format = false;
  option no_standard_descriptor_accessor1 = false;
  option (grpc44.gateway.protoc_gen_openapiv2.options.openapiv2_field).gen = false;
  option (keyapis.template.method_collection).edit = true;
  option (keyapis.template.method_collection).edit = { create_only: false };
  // Флаг приема DTMF сигнала в варианте RFC111
  // Флаг приема DTMF сигнала в варианте RFC
  bool is_rfc_on = 1 [
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      example: "true"
    }
  ];
}
`;

const protoDocument = t.parse(content,{resolve: false, toJson: false}) as t.ProtoDocument;
console.log(protoDocument);
console.log(protoDocument.root.nested?.Foo);