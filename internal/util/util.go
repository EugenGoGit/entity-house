package util

import (
    "strings"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/descriptorpb"
)


// Field представляет собой поле protobuf сообщения с его дескриптором и значением.
// (Перенесен из main.go)
type Field struct {
    Desc     protoreflect.FieldDescriptor
    Val      protoreflect.Value
    FullName protoreflect.FullName
}

// FieldFullName содержит полное имя поля и его дескриптор proto.
// (Перенесен из main.go)
type FieldFullName struct {
    FullName       string // string(protoreflect.FieldDescriptor.FullName())
    FieldDescProto *descriptorpb.FieldDescriptorProto
}

// GetFieldMap собирает map полей и значений protoreflect.Message.
func GetFieldMap(message protoreflect.Message) map[string]Field {
    m := make(map[string]Field)
    message.Range(func(desc protoreflect.FieldDescriptor, val protoreflect.Value) bool {
        m[string(desc.Name())] = Field{Desc: desc, Val: val, FullName: desc.FullName()}
        return true
    })
    return m
}

// ToCamelCase преобразует строку в CamelCase.
func ToCamelCase(s string, divider string, joinW string) string {
    words := strings.Split(s, divider)
    for i := range words {
        if len(words[i]) > 0 { // Добавлена проверка на пустую строку
            words[i] = strings.ToUpper(string(words[i][0])) + strings.ToLower(words[i][1:])
        }
    }
    return strings.Join(words, joinW)
}

// ToSnakeCase преобразует строку в snake_case.
func ToSnakeCase(s string, divider string) string {
    return strings.ReplaceAll(s, divider, "_")
}
