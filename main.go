package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	// "unicode"

	// "path/filepath"
	"strings"

	//     "google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile"
	// "github.com/bufbuild/protocompile/linker"
	// "github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/reporter"
	"google.golang.org/protobuf/reflect/protodesc"

	// "google.golang.org/protobuf/encoding/prototext"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/builder"
	"github.com/jhump/protoreflect/desc/protoprint"
	"google.golang.org/protobuf/proto"
	pref "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	// "google.golang.org/protobuf/types/dynamicpb"
	// "github.com/bufbuild/protocompile/protoutil"
	// "google.golang.org/protobuf/encoding/protojson"
	"maps"
)

// Собираем map полей и значений protoreflect.Message
type Field struct {
	desc     pref.FieldDescriptor
	val      pref.Value
	fullName pref.FullName
}

func getFieldMap(message pref.Message) map[string]Field {
	var m map[string]Field = make(map[string]Field)
	message.Range(func(Desc pref.FieldDescriptor, Val pref.Value) bool {
		m[string(Desc.Name())] = Field{Desc, Val, Desc.FullName()}
		return true
	})
	return m
}

func printEnumEl(el Field) {
	fmt.Println(el.desc.FullName(), " ", el.desc.Enum().FullName(), " ", el.val, " ", el.desc.Enum().Values().Get(int(el.val.Enum())).Name())
	fmt.Println()
}

func createTypeDescByTemplateParent(
	templateDesc pref.MessageDescriptor,
	// templateParent *pref.Value,
	messageName string,
	messageFullName string,
	entityTypeName string,
	entityPrefDesc pref.MessageDescriptor,
	entityKeyFields []FieldFullName,
	methodDescFields map[string]Field,
	fileCommentsMap map[string]string,
	addMessageToProtoRoot map[string]*descriptorpb.DescriptorProto,
	linkedTypeName string,
) (*descriptorpb.DescriptorProto, error) {
	resultProtodesc := protodesc.ToDescriptorProto(templateDesc)
	resultProtodesc.Name = &messageName
	err := getTypeDescByTemplate(
		resultProtodesc,
		templateDesc,
		entityTypeName,
		entityKeyFields,
		string(templateDesc.FullName()),
		methodDescFields,
		messageName,
		string(entityPrefDesc.ParentFile().Package()),
		messageFullName,
		fileCommentsMap,
		addMessageToProtoRoot,
		linkedTypeName,
	)
	if err != nil {
		return nil, err
	}
	return resultProtodesc, nil
}

func getTypeDescByTemplate(
	resultDescProto *descriptorpb.DescriptorProto,
	templatePrefDesc pref.MessageDescriptor,
	entityTypeName string,
	entityKeyFields []FieldFullName,
	templateTypeName string,
	methodDescFields map[string]Field,
	parentNameToType string,
	packageNameToType string,
	parentFullNameToType string,
	fileCommentsMap map[string]string,
	addMessageToProtoRoot map[string]*descriptorpb.DescriptorProto,
	linkedTypeName string,
) error {
	var i int32 = 0
	for len(resultDescProto.Field) > int(i) {
		fieldPrefDesc := templatePrefDesc.Fields().ByName(pref.Name(*resultDescProto.Field[i].Name))
		if val, ok := getFieldMap(fieldPrefDesc.Options().ProtoReflect())["replace_field_type_to"]; ok {
			if val.val.String() == "EntityKeyFields" {

				// Вставляем ключи с сущности и комменты с ключей с сущности
				if len(entityKeyFields) == 0 {
					panic(errors.New("Не удалось найти ключевые поля сущности " + packageNameToType + "." + entityTypeName))
				}
				entityKeyFields[0].fieldDescProto.Number = resultDescProto.Field[i].Number
				opts := getFieldMap(resultDescProto.Field[i].Options.ProtoReflect())
				for _, v := range opts {
					entityKeyFields[0].fieldDescProto.GetOptions().ProtoReflect().NewField(v.desc)
					entityKeyFields[0].fieldDescProto.GetOptions().ProtoReflect().Set(v.desc, v.val)
				}

				resultDescProto.Field[i] = entityKeyFields[0].fieldDescProto
				fileCommentsMap[string(parentFullNameToType)+"."+resultDescProto.Field[i].GetName()] = fileCommentsMap[entityKeyFields[0].fullName]

				for j := 1; j < len(entityKeyFields); j++ {
					n := *entityKeyFields[j-1].fieldDescProto.Number + int32(1)
					entityKeyFields[j].fieldDescProto.Number = &n
					for _, v := range opts {
						entityKeyFields[j].fieldDescProto.GetOptions().ProtoReflect().NewField(v.desc)
						entityKeyFields[j].fieldDescProto.GetOptions().ProtoReflect().Set(v.desc, v.val)
					}
					// replaceKeysToFields[j].fieldDescProto.Options = *opts
					if len(resultDescProto.Field) == int(i+1) {
						resultDescProto.Field = append(resultDescProto.Field, entityKeyFields[j].fieldDescProto)
					} else {
						resultDescProto.Field = append(resultDescProto.Field[:i+1], resultDescProto.Field[i:]...) // index < len(a)
						resultDescProto.Field[i+1] = entityKeyFields[j].fieldDescProto
					}
					i = i + 1
					fileCommentsMap[string(parentFullNameToType)+"."+resultDescProto.Field[i].GetName()] = fileCommentsMap[entityKeyFields[j].fullName]
				}
				// Проставим номера полей на случай пересечения номеров
				for j := range resultDescProto.Field {
					n := int32(j + 1)
					if n > *resultDescProto.Field[j].Number {
						resultDescProto.Field[j].Number = &n
					}
				}
			} else {
				// Меняем тип поля на тип сущности и добавляем коммент от сущности
				replaceToType := strings.Replace(val.val.String(), "{EntityTypeName}", entityTypeName, -1)
				replaceToType = strings.Replace(replaceToType, "{LinkedTypeName}", linkedTypeName, -1)
				resultDescProto.Field[i].TypeName = &replaceToType

				if *resultDescProto.Field[i].Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
					fieldTypePrefDesc := fieldPrefDesc.Message()
					fieldTypeOpts := getFieldMap(fieldTypePrefDesc.Options().ProtoReflect())
					msgDescProto := protodesc.ToDescriptorProto(fieldTypePrefDesc)
					if val, ok := fieldTypeOpts["replace_message_type_name_to"]; ok {
						msgDescProto.Options.ProtoReflect().Clear(val.desc)
						s := strings.Replace(val.val.String(), "{EntityTypeName}", entityTypeName, -1)
						s = strings.Replace(s, "{LinkedTypeName}", linkedTypeName, -1)
						msgDescProto.Name = &s
						err := getTypeDescByTemplate(
							msgDescProto,
							fieldTypePrefDesc,
							entityTypeName,
							entityKeyFields,
							string(fieldTypePrefDesc.FullName()),
							methodDescFields,
							string(*msgDescProto.Name),
							packageNameToType,
							packageNameToType+"."+msgDescProto.GetName(),
							fileCommentsMap,
							addMessageToProtoRoot,
							linkedTypeName,
						)
						if err != nil {
							return err
						}

						addMessageToProtoRoot[string(fieldTypePrefDesc.Name())] = msgDescProto

					}
					// Добавим коммент для него
					if val, ok := fieldTypeOpts["message_comments"]; ok {
						fileCommentsMap[packageNameToType+"."+msgDescProto.GetName()] = strings.Replace(val.val.String(), "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
						fileCommentsMap[packageNameToType+"."+msgDescProto.GetName()] = strings.Replace(fileCommentsMap[packageNameToType+"."+msgDescProto.GetName()], "{LinkedTypeName}", linkedTypeName, -1)
						msgDescProto.GetOptions().ProtoReflect().Clear(val.desc)
					}
				}
			}

			// Если есть коммент на тип поля его на поле, если нет - ставим комменты из опции field_comments поля в шаблоне
			// убираем опцию
			if typeComment, ok := fileCommentsMap[packageNameToType+"."+resultDescProto.Field[i].GetTypeName()]; ok {
				fileCommentsMap[parentFullNameToType+"."+resultDescProto.Field[i].GetName()] = typeComment
				if val, ok := getFieldMap(resultDescProto.Field[i].Options.ProtoReflect())["field_comments"]; ok {
					resultDescProto.Field[i].GetOptions().ProtoReflect().Clear(val.desc)
				}
			} else {
				if val, ok := getFieldMap(resultDescProto.Field[i].Options.ProtoReflect())["field_comments"]; ok {
					comment := strings.Replace(val.val.String(), "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
					comment = strings.Replace(comment, "{LinkedTypeName}", linkedTypeName, -1)
					comment = strings.Replace(comment, "{EntityTypeName}", entityTypeName, -1)
					fileCommentsMap[parentFullNameToType+"."+resultDescProto.Field[i].GetName()] = comment
					resultDescProto.Field[i].GetOptions().ProtoReflect().Clear(val.desc)
				}
			}
			// убираем опцию replace_field_type_to
			resultDescProto.Field[i].GetOptions().ProtoReflect().Clear(val.desc)
		} else {
			if *resultDescProto.Field[i].Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE ||
				*resultDescProto.Field[i].Type == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
				// заменяем тип на тип поля в результате применения шаблона
				s := strings.Replace(*resultDescProto.Field[i].TypeName, "."+templateTypeName+".", parentNameToType+".", -1)
				// Если замены не произошло, значит тип не в Nested типах шаблона и нужно добавить его в генерацию отдельно от результата шаблона
				if s == *resultDescProto.Field[i].TypeName {
					if *resultDescProto.Field[i].Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
						// resParentFullNameToType = packageNameToType
						msgDescProto := protodesc.ToDescriptorProto(templatePrefDesc.Fields().Get(int(i)).Message())
						err := getTypeDescByTemplate(
							msgDescProto,
							templatePrefDesc.Fields().Get(int(i)).Message(),
							entityTypeName,
							entityKeyFields,
							string(templatePrefDesc.Fields().Get(int(i)).Message().FullName()),
							methodDescFields,
							string(templatePrefDesc.Fields().Get(int(i)).Message().Name()),
							packageNameToType,
							string(templatePrefDesc.ParentFile().Package())+"."+msgDescProto.GetName(),
							fileCommentsMap,
							addMessageToProtoRoot,
							linkedTypeName,
						)
						if err != nil {
							return err
						}
						// Добавим этот тип в генерацию и коммент для него
						if val, ok := getFieldMap(msgDescProto.GetOptions().ProtoReflect())["message_comments"]; ok {
							fileCommentsMap[packageNameToType+"."+msgDescProto.GetName()] = strings.Replace(val.val.String(), "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
							fileCommentsMap[packageNameToType+"."+msgDescProto.GetName()] = strings.Replace(fileCommentsMap[packageNameToType+"."+msgDescProto.GetName()], "{LinkedTypeName}", linkedTypeName, -1)
							msgDescProto.GetOptions().ProtoReflect().Clear(val.desc)
						}
						addMessageToProtoRoot[string(templatePrefDesc.Fields().Get(int(i)).Message().Name())] = msgDescProto
						s = string(templatePrefDesc.Fields().Get(int(i)).Message().Name())
					}
				}
				// ставим комменты из опции field_comments поля в шаблоне, убираем опцию
				if val, ok := getFieldMap(resultDescProto.Field[i].Options.ProtoReflect())["field_comments"]; ok {
					fileCommentsMap[parentFullNameToType+"."+resultDescProto.Field[i].GetName()] = strings.Replace(val.val.String(), "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
					fileCommentsMap[parentFullNameToType+"."+resultDescProto.Field[i].GetName()] = strings.Replace(fileCommentsMap[parentFullNameToType+"."+resultDescProto.Field[i].GetName()], "{LinkedTypeName}", linkedTypeName, -1)
					resultDescProto.Field[i].GetOptions().ProtoReflect().Clear(val.desc)
				}
				resultDescProto.Field[i].TypeName = &s
			} else {
				// Поле не заменяем, скалярный тип оставляем, ставим комменты из опции field_comments поля в шаблоне, убираем опцию
				if val, ok := getFieldMap(resultDescProto.Field[i].Options.ProtoReflect())["field_comments"]; ok {
					fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.Field[i].Name] = strings.Replace(val.val.String(), "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
					fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.Field[i].Name] = strings.Replace(fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.Field[i].Name], "{LinkedTypeName}", linkedTypeName, -1)
					resultDescProto.Field[i].GetOptions().ProtoReflect().Clear(val.desc)
				}
			}
		}
		i = i + 1
	}

	for i := range resultDescProto.EnumType {

		enumOpts := getFieldMap(resultDescProto.EnumType[i].Options.ProtoReflect())
		// Для enum с опцией enum_by_method_attribute дополняем элементы из указанного поля опций метода
		if methodAttribute, ok := enumOpts["enum_by_method_attribute"]; ok {
			if enumValues, ok := methodDescFields[methodAttribute.val.String()]; ok {
				fmt.Println("enumValues", enumValues.val)
				for n := range enumValues.val.List().Len() {
					enumValue := getFieldMap(enumValues.val.List().Get(n).Message())
					name := enumValue["name"].val.String()
					title := enumValue["title"].val.String()
					number := int32(len(resultDescProto.EnumType[i].Value))

					resultDescProto.EnumType[i].Value = append(
						resultDescProto.EnumType[i].Value,
						&descriptorpb.EnumValueDescriptorProto{Name: &name,
							Number: &number})
					fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.EnumType[i].Name+"."+name] = strings.Replace(title, "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
					fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.EnumType[i].Name+"."+name] = strings.Replace(fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.EnumType[i].Name+"."+name], "{LinkedTypeName}", linkedTypeName, -1)

				}
			}
			// fmt.Println("rresultDescProto.EnumType[i].Value", resultDescProto.EnumType[i].Value)
			// убираем опцию enum_by_method_attribute
			resultDescProto.EnumType[i].Options.ProtoReflect().Clear(methodAttribute.desc)
		}
		// Добавим комменты
		if commentDesc, ok := enumOpts["enum_comments"]; ok {
			fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.EnumType[i].Name] = strings.Replace(commentDesc.val.String(), "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
			fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.EnumType[i].Name] = strings.Replace(fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.EnumType[i].Name], "{LinkedTypeName}", linkedTypeName, -1)
			// убираем опцию
			resultDescProto.EnumType[i].Options.ProtoReflect().Clear(commentDesc.desc)
		}
		// Добавим комменты значений перечисления
		for j := range resultDescProto.EnumType[i].Value {
			enumValOpts := getFieldMap(resultDescProto.EnumType[i].GetValue()[j].Options.ProtoReflect())
			if commentDesc, ok := enumValOpts["enum_value_comments"]; ok {
				fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.EnumType[i].Name+"."+*resultDescProto.EnumType[i].GetValue()[j].Name] = strings.Replace(commentDesc.val.String(), "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
				fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.EnumType[i].Name+"."+*resultDescProto.EnumType[i].GetValue()[j].Name] = strings.Replace(fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*resultDescProto.EnumType[i].Name+"."+*resultDescProto.EnumType[i].GetValue()[j].Name], "{LinkedTypeName}", linkedTypeName, -1)
				// убираем опцию
				resultDescProto.EnumType[i].GetValue()[j].Options.ProtoReflect().Clear(commentDesc.desc)
			}
		}
	}

	// Ставим коммент из опции message_comments и уберем эту опцию
	// поменяем тип на нового парента
	// рекурсивно меняем вложенные типы
	for i := range resultDescProto.NestedType {
		if val, ok := getFieldMap(resultDescProto.NestedType[i].GetOptions().ProtoReflect())["message_comments"]; ok {
			fileCommentsMap[parentFullNameToType+"."+resultDescProto.NestedType[i].GetName()] = strings.Replace(val.val.String(), "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
			fileCommentsMap[parentFullNameToType+"."+resultDescProto.NestedType[i].GetName()] = strings.Replace(fileCommentsMap[parentFullNameToType+"."+resultDescProto.NestedType[i].GetName()], "{LinkedTypeName}", linkedTypeName, -1)
			resultDescProto.NestedType[i].GetOptions().ProtoReflect().Clear(val.desc)
		}
		s := strings.Replace(*resultDescProto.NestedType[i].Name, templateTypeName, parentFullNameToType, -1)
		resultDescProto.NestedType[i].Name = &s

		err := getTypeDescByTemplate(
			resultDescProto.NestedType[i],
			templatePrefDesc.Messages().ByName(pref.Name(*resultDescProto.NestedType[i].Name)),
			entityTypeName,
			entityKeyFields,
			templateTypeName,
			methodDescFields,
			parentNameToType,
			packageNameToType,
			parentFullNameToType+"."+resultDescProto.NestedType[i].GetName(),
			fileCommentsMap,
			addMessageToProtoRoot,
			linkedTypeName,
		)
		if err != nil {
			return err
		}
	}

	// Ставим коммент из опции oneof_comments и уберем эту опцию
	for i := range resultDescProto.GetOneofDecl() {
		if val, ok := getFieldMap(resultDescProto.GetOneofDecl()[i].GetOptions().ProtoReflect())["oneof_comments"]; ok {
			fileCommentsMap[string(parentFullNameToType)+"."+resultDescProto.GetOneofDecl()[i].GetName()] = strings.Replace(val.val.String(), "{EntityTypeComment}", fileCommentsMap[packageNameToType+"."+entityTypeName], -1)
			fileCommentsMap[string(parentFullNameToType)+"."+resultDescProto.GetOneofDecl()[i].GetName()] = strings.Replace(fileCommentsMap[string(parentFullNameToType)+"."+resultDescProto.GetOneofDecl()[i].GetName()], "{LinkedTypeName}", linkedTypeName, -1)
			resultDescProto.GetOneofDecl()[i].GetOptions().ProtoReflect().Clear(val.desc)
		}
	}
	return nil
}

func getUniqueFieldGroup(entityDesc pref.MessageDescriptor, uniqueFieldGroupEl pref.EnumNumber) []FieldFullName {
	var res []FieldFullName
	var m map[pref.EnumNumber][]FieldFullName = make(map[pref.EnumNumber][]FieldFullName)
	var min pref.EnumNumber = 10000
	for i := range entityDesc.Fields().Len() {
		optMap := getFieldMap(entityDesc.Fields().Get(i).Options().ProtoReflect())
		if val, ok := optMap["unique_field_group"]; ok {
			m[val.val.Enum()] = append(m[val.val.Enum()], FieldFullName{fullName: string(entityDesc.Fields().Get(i).FullName()), fieldDescProto: protodesc.ToFieldDescriptorProto(entityDesc.Fields().Get(i))})
			if val.val.Enum() < min {
				min = val.val.Enum()
			}
		}
	}
	if val, ok := m[uniqueFieldGroupEl]; ok {
		res = val
	} else {
		res = m[min]
	}
	return res
}

type FieldFullName struct {
	fullName       string
	fieldDescProto *descriptorpb.FieldDescriptorProto
}

func getKeyFields(keyFieldsDefinition *pref.Value, entityDesc pref.MessageDescriptor) []FieldFullName {
	var res []FieldFullName
	if keyFieldsDefinition.String() == "<nil>" {
		res = getUniqueFieldGroup(entityDesc, 0)
	} else {
		fieldMap := getFieldMap(keyFieldsDefinition.Message())
		if val, ok := fieldMap["key_field_list"]; ok {
			keyFieldsFieldMap := getFieldMap(val.val.Message())
			if val1, ok := keyFieldsFieldMap["key_fields"]; ok {
				for i := range val1.val.List().Len() {
					res = append(res, FieldFullName{
						fullName: string(entityDesc.Fields().ByName(pref.Name(val1.val.List().Get(i).String())).FullName()),
						fieldDescProto: protodesc.ToFieldDescriptorProto(
							entityDesc.Fields().ByName(pref.Name(val1.val.List().Get(i).String())),
						),
					})
				}
			} else {
				res = getUniqueFieldGroup(entityDesc, 0)
			}
		} else {
			if val1, ok := fieldMap["unique_field_group"]; ok {
				res = getUniqueFieldGroup(entityDesc, val1.val.Enum())
			} else {
				res = getUniqueFieldGroup(entityDesc, 0)
			}
		}
	}
	return res
}

func getProtoJ(c protocompile.Compiler, fd *descriptorpb.FileDescriptorProto, appendTo map[string]*desc.FileDescriptor) *desc.FileDescriptor {
	var dpjAr []*desc.FileDescriptor
	for i := range fd.Dependency {
		if val, ok := appendTo[fd.Dependency[i]]; ok {
			dpjAr = append(dpjAr, val)
			appendTo[fd.Dependency[i]] = val
		} else {
			fdr, err := c.Compile(context.Background(), fd.Dependency[i])
			if err != nil {
				panic(err)
			}
			dp := protodesc.ToFileDescriptorProto(fdr[0])
			dpj := getProtoJ(c, dp, appendTo)

			dpjAr = append(dpjAr, dpj)
			appendTo[fd.Dependency[i]] = dpj
		}
	}
	res, err := desc.CreateFileDescriptor(fd, dpjAr...)
	if err != nil {
		panic(err)
	}
	return res
}

func getMessageComments(messJhumpDesc *desc.MessageDescriptor, commentsMap map[string]string) {
	if messJhumpDesc.GetSourceInfo().LeadingComments != nil {
		commentsMap[messJhumpDesc.GetFullyQualifiedName()] = *messJhumpDesc.GetSourceInfo().LeadingComments
	}

	for i := range messJhumpDesc.GetNestedMessageTypes() {
		getMessageComments(messJhumpDesc.GetNestedMessageTypes()[i],
			commentsMap,
		)
	}
	for i := range messJhumpDesc.GetFields() {
		if messJhumpDesc.GetFields()[i].GetSourceInfo().LeadingComments != nil {
			commentsMap[messJhumpDesc.GetFullyQualifiedName()+"."+messJhumpDesc.GetFields()[i].GetName()] = *messJhumpDesc.GetFields()[i].GetSourceInfo().LeadingComments

		}
	}
	for i := range messJhumpDesc.GetOneOfs() {
		if messJhumpDesc.GetOneOfs()[i].GetSourceInfo().LeadingComments != nil {
			commentsMap[messJhumpDesc.GetFullyQualifiedName()+"."+messJhumpDesc.GetOneOfs()[i].GetName()] = *messJhumpDesc.GetOneOfs()[i].GetSourceInfo().LeadingComments

		}
	}
}

func fillMessageComments(messJhumpDesc *desc.MessageDescriptor, messBuilder *builder.MessageBuilder, commentsMap map[string]string) {
	if val, ok := commentsMap[messJhumpDesc.GetFullyQualifiedName()]; ok {
		messBuilder.SetComments(builder.Comments{LeadingComment: val})
	}
	for i := range messJhumpDesc.GetNestedMessageTypes() {
		fillMessageComments(messJhumpDesc.GetNestedMessageTypes()[i],
			messBuilder.GetNestedMessage(messJhumpDesc.GetNestedMessageTypes()[i].GetName()),
			commentsMap,
		)
	}
	for i := range messJhumpDesc.GetFields() {
		if val, ok := commentsMap[messJhumpDesc.GetFields()[i].GetFullyQualifiedName()]; ok {
			messBuilder.GetField(messJhumpDesc.GetFields()[i].GetName()).SetComments(builder.Comments{LeadingComment: val})
		}
	}

	for i := range messJhumpDesc.GetOneOfs() {
		if val, ok := commentsMap[messJhumpDesc.GetOneOfs()[i].GetFullyQualifiedName()]; ok {
			messBuilder.GetOneOf(messJhumpDesc.GetOneOfs()[i].GetName()).SetComments(builder.Comments{LeadingComment: val})
		}
	}
	for i := range messJhumpDesc.GetNestedEnumTypes() {
		if val, ok := commentsMap[messJhumpDesc.GetNestedEnumTypes()[i].GetFullyQualifiedName()]; ok {
			messBuilder.GetNestedEnum(messJhumpDesc.GetNestedEnumTypes()[i].GetName()).SetComments(builder.Comments{LeadingComment: val})
		}
		for j := range messJhumpDesc.GetNestedEnumTypes()[i].GetValues() {
			if val, ok := commentsMap[messJhumpDesc.GetNestedEnumTypes()[i].GetValues()[j].GetFullyQualifiedName()]; ok {
				messBuilder.GetNestedEnum(messJhumpDesc.GetNestedEnumTypes()[i].GetName()).GetValue(messJhumpDesc.GetNestedEnumTypes()[i].GetValues()[j].GetName()).SetComments(builder.Comments{LeadingComment: val})
			}
		}
	}

}

func getDtoTemplDesc(containerFieldName string, varFieldMap, tmplFieldMap map[string]Field, varMethod pref.Message) (pref.MessageDescriptor, error) {
	var requestTemplateDesc pref.MessageDescriptor
	// Берем Шаблон из спецификации в описании сущности
	if val, ok := varFieldMap[containerFieldName]; ok {
		requestTemplateFieldMap := getFieldMap(val.val.Message())
		if len(requestTemplateFieldMap) > 0 {
			// Тут только одно поле DTO
			for _, v := range getFieldMap(val.val.Message()) {
				requestTemplateDesc = v.val.Message().Descriptor()
				break
			}
		}
	}
	// Если Шаблон не задан в спецификации описании сущности, то берем указанный в шаблоне
	if requestTemplateDesc == nil {
		requestTemplateDesc = varMethod.Descriptor().Fields().ByName(pref.Name(containerFieldName)).Message().Fields().ByName(
			pref.Name(getFieldMap(tmplFieldMap[containerFieldName].val.Message())["spec_field_name"].val.String())).Message()
	}
	return requestTemplateDesc, nil
}

func genMethod(
	tmplMethod pref.Message,
	varMethod pref.Message,
	serviceKeyFieldsDefinition pref.Value,
	genFileProto *descriptorpb.FileDescriptorProto,
	entityPrefDesc pref.MessageDescriptor,
	entityMessageProtodesc *descriptorpb.DescriptorProto,
	genFileComments map[string]string,
	addProtoRoot map[string]*descriptorpb.DescriptorProto,
	httpRoot string,
	googleApiAnnotationsPrefDesc pref.FileDescriptor,
	genServiceProto *descriptorpb.ServiceDescriptorProto,
	serviceName string,

) error {
	var methodParameters map[string]Field = make(map[string]Field)
	tmplFieldMap := getFieldMap(tmplMethod)
	varFieldMap := getFieldMap(varMethod)
	// Атрибуты, которые определяются в шаблоне спецификации
	maps.Copy(methodParameters, getFieldMap(tmplFieldMap["attributes"].val.Message()))
	maps.Copy(methodParameters, getFieldMap(methodParameters["comments"].val.Message()))

	// Атрибуты, заданные в описании сущности переопределяют те, что заданы в шаблоне
	maps.Copy(methodParameters, varFieldMap)
	maps.Copy(methodParameters, getFieldMap(methodParameters["comments"].val.Message()))

	requestTemplateDesc, err :=
		getDtoTemplDesc(
			"request_template",
			varFieldMap,
			tmplFieldMap,
			varMethod,
		)
	if err != nil {
		return err
	}
	responseTemplateDesc, err :=
		getDtoTemplDesc(
			"response_template",
			varFieldMap,
			tmplFieldMap,
			varMethod,
		)
	if err != nil {
		return err
	}

	var linkedTypeName string
	var linkedTypeKeyFieldPath string
	if val, ok := methodParameters["linked_type"]; ok {
		linkedType := getFieldMap(val.val.Message())
		linkedTypeName = linkedType["name"].val.String()
		linkedTypeKeyFieldPath = linkedType["key_field_path"].val.String()
	}

	// Определение уникальных полей сервиса
	// TODO: переопределить с сервиса
	keyFieldsDefinition := serviceKeyFieldsDefinition
	keyFieldsDefinition = methodParameters["key_fields_definition"].val

	methodName := methodParameters["name"].val.String()
	methodName = strings.Replace(methodName, "{EntityTypeName}", string(entityPrefDesc.Name()), -1)
	methodName = strings.Replace(methodName, "{LinkedTypeName}", linkedTypeName, -1)

	// шаблон имени запроса в request_name
	requestName := methodParameters["request_name"].val.String()
	requestName = strings.Replace(requestName, "{EntityTypeName}", string(entityPrefDesc.Name()), -1)
	requestName = strings.Replace(requestName, "{LinkedTypeName}", linkedTypeName, -1)
	requestFullName := *genFileProto.Package + "." + requestName

	// шаблон имени ответа в response_name
	responseName := methodParameters["response_name"].val.String()
	responseName = strings.Replace(responseName, "{EntityTypeName}", string(entityPrefDesc.Name()), -1)
	responseName = strings.Replace(responseName, "{LinkedTypeName}", linkedTypeName, -1)
	responseFullName := *genFileProto.Package + "." + responseName

	// определяем список ключевых полей для метода
	// Переопределяем этот параметр сервиса
	keyFieldList := getKeyFields(&keyFieldsDefinition, entityPrefDesc)

	// // шаблон запроса в request_template
	// requestTmpl := methodParameters["request_template"]

	requestMessageProtodesc, err := createTypeDescByTemplateParent(
		requestTemplateDesc,
		requestName,
		requestFullName,
		*entityMessageProtodesc.Name,
		entityPrefDesc,
		keyFieldList,
		methodParameters,
		genFileComments,
		addProtoRoot,
		linkedTypeName,
	)
	if err != nil {
		return err
	}

	// responseTmpl := methodParameters["response_template"]

	responseMessageProtodesc, err := createTypeDescByTemplateParent(
		responseTemplateDesc,
		responseName,
		responseFullName,
		*entityMessageProtodesc.Name,
		entityPrefDesc,
		keyFieldList,
		methodParameters,
		genFileComments,
		addProtoRoot,
		linkedTypeName,
	)
	if err != nil {
		return err
	}

	genFileProto.MessageType = append(genFileProto.MessageType, requestMessageProtodesc)
	genFileProto.MessageType = append(genFileProto.MessageType, responseMessageProtodesc)

	genMethodProto := &descriptorpb.MethodDescriptorProto{
		Name:            proto.String(methodName),
		InputType:       proto.String(requestName),
		OutputType:      proto.String(responseName),
		ServerStreaming: proto.Bool(true),
		ClientStreaming: proto.Bool(true),
		Options:         &descriptorpb.MethodOptions{},
	}

	// в поле server_streaming признак ответа в потоке
	var serverStreaming bool
	if val, ok := methodParameters["server_streaming"]; ok {
		serverStreaming = val.val.Bool()
	} else {
		serverStreaming = false
	}
	genMethodProto.ServerStreaming = &serverStreaming

	// в поле client_streaming признак ответа в потоке
	var clientStreaming bool
	if val, ok := methodParameters["client_streaming"]; ok {
		clientStreaming = val.val.Bool()
	} else {
		clientStreaming = false
	}
	genMethodProto.ClientStreaming = &clientStreaming

	// в полях entity.api.http_rule http опции google.api.HttpRule
	// если заданы http_rule и httpRoot, то добавляем опции google.api.http для этого метода
	// с заменой по шаблону
	var httpRule Field
	isHttpRule := false
	if fieldHttpRule, ok := methodParameters["http_rule"]; ok {
		httpRule = fieldHttpRule
		isHttpRule = true
	}
	if isHttpRule {
		if httpRoot != "" && httpRoot != "<nil>" {
			var keyFieldPath string
			if len(keyFieldList) == 1 {
				keyFieldPath = keyFieldPath + "{" + *keyFieldList[0].fieldDescProto.Name + "}"
			} else {
				for i := range keyFieldList {
					keyFieldPath = keyFieldPath + *keyFieldList[i].fieldDescProto.Name + "/{" + *keyFieldList[i].fieldDescProto.Name + "}"
					if i != len(keyFieldList)-1 {
						keyFieldPath = keyFieldPath + "/"
					}
				}
			}
			methodHttpRuleMap := getFieldMap(httpRule.val.Message())
			var valOpt string = "{ "

			fdHttp := googleApiAnnotationsPrefDesc.Extensions().ByName("http")
			fdHttpV := dynamicpb.NewMessage(fdHttp.Message())
			for k, v := range methodHttpRuleMap {
				fd := fdHttp.Message().Fields().ByName(pref.Name(k))
				httpPath := strings.Replace(v.val.String(), "{HttpRoot}", httpRoot, -1)
				httpPath = strings.Replace(httpPath, "{KeyFields}", keyFieldPath, -1)
				httpPath = strings.Replace(httpPath, "{LinkKeyFieldPath}", linkedTypeKeyFieldPath, -1)
				httpPath = strings.Replace(httpPath, "{LinkedTypeName}", strings.ToLower(linkedTypeName), -1)
				fdHttpV.Set(fd, pref.ValueOf(httpPath))
				valOpt = valOpt + k + ": \"" +
					// strings.Replace(strings.Replace(v.val.String(), "{HttpRoot}", httpRoot, -1), "{KeyFields}", keyFieldPath, -1) +
					httpPath + "\", "
			}
			genMethodProto.Options.ProtoReflect().Set(fdHttp, pref.ValueOf(fdHttpV))
		}
	}

	// Добавляем метод с комментарием
	// Добавляем комментарии к запросу и ответу метода
	genServiceProto.Method = append(genServiceProto.Method, genMethodProto)
	var methodComment string
	if val, ok := methodParameters["leading_comment"]; ok {
		fmt.Println("methodParameters[leading_comment]", methodParameters["leading_comment"].val.String())
		methodComment = strings.Replace(val.val.String(), "{EntityTypeName}", string(entityPrefDesc.Name()), -1)
		methodComment = strings.Replace(methodComment, "{LinkedTypeName}", linkedTypeName, -1)
	}
	fmt.Println("methodParameters[leading_comment]", methodComment)
	var requestComment string
	var responseComment string

	if val, ok := methodParameters["additional_leading_comment"]; ok {
		methodComment = methodComment + ".\n" + val.val.String()
	}
	if val, ok := methodParameters["request_leading_comment"]; ok {
		requestComment = val.val.String()
		requestComment = strings.Replace(requestComment, "{EntityTypeName}", string(entityPrefDesc.Name()), -1)
		requestComment = strings.Replace(requestComment, "{LinkedTypeName}", linkedTypeName, -1)
	}
	if val, ok := methodParameters["response_leading_comment"]; ok {
		responseComment = val.val.String()
		responseComment = strings.Replace(responseComment, "{EntityTypeName}", string(entityPrefDesc.Name()), -1)
		responseComment = strings.Replace(responseComment, "{LinkedTypeName}", linkedTypeName, -1)
	}

	genFileComments[*genFileProto.Package+"."+serviceName+"."+methodName] = methodComment
	genFileComments[*genFileProto.Package+"."+requestName] = requestComment
	genFileComments[*genFileProto.Package+"."+responseName] = responseComment
	return nil
}

func genEntityApiSpec(apiSpecOpt Field,
	entityPrefDesc pref.MessageDescriptor,
	genFileProto *descriptorpb.FileDescriptorProto,
	googleApiAnnotationsPrefDesc pref.FileDescriptor,
	genFileComments map[string]string,
	addProtoRoot map[string]*descriptorpb.DescriptorProto,
) error {
	// Возьмем описание сервиса из опции сущности
	// Считаем, что там только один сервис
	// TODO: обработать несколько сервисов
	var entitySourceService Field
	for _, v := range getFieldMap(apiSpecOpt.val.Message()) {
		entitySourceService = v
		break
	}
	entitySourceServiceFieldMap := getFieldMap(entitySourceService.val.Message())

	// Возьмем описание сервиса из имплементации шаблона спецификации
	// Считаем, что там только один сервис
	// TODO: обработать несколько сервисов
	serviceTmplList := getFieldMap(getFieldMap(apiSpecOpt.val.Message().Descriptor().Options().ProtoReflect())["specification_tmpl"].val.Message())["service_tmpl"].val.List()
	var tmplServiceFieldMap map[string]Field
	for i := range serviceTmplList.Len() {
		tmplServiceFieldMap = getFieldMap(serviceTmplList.Get(i).Message())
		if tmplServiceFieldMap["spec_field_name"].val.String() == string(entitySourceService.desc.Name()) {
			break
		}
	}
	fmt.Println("tmplService", getFieldMap(getFieldMap(apiSpecOpt.val.Message().Descriptor().Options().ProtoReflect())["specification_tmpl"].val.Message()))
	fmt.Println("tmplService", tmplServiceFieldMap)
	var serviceParameters map[string]Field = make(map[string]Field)
	// Атрибуты, которые определяются в шаблоне спецификации
	maps.Copy(serviceParameters, getFieldMap(tmplServiceFieldMap["attributes"].val.Message()))

	// Добавим атрибуты из описания спецификации на сущности
	// При совпадении имени они переопределят атрибуты из шаблона
	// Атрибуты, которые нельзя задать в шаблоне спецификации, а только в описании сущности
	maps.Copy(serviceParameters, entitySourceServiceFieldMap)

	serviceName := serviceParameters["name"].val.String()
	serviceComment := serviceParameters["leading_comment"].val.String()
	if val, ok := serviceParameters["additional_leading_comment"]; ok {
		serviceComment = serviceComment + ".\n" + val.val.String()
	}

	// TODO: Универсальная процедура подстановок
	serviceName = strings.Replace(serviceName, "{EntityTypeName}", string(entityPrefDesc.Name()), -1)
	serviceComment = strings.Replace(serviceComment, "{EntityTypeName}", string(entityPrefDesc.Name()), -1)
	// Определение уникальных полей сервиса, будет использовано, если не переопределено на уровне метода
	serviceKeyFieldsDefinition := serviceParameters["http_root"].val
	// Корневой httpPath в http_root, он же - признак добавления опций http
	httpRoot := serviceParameters["http_root"].val.String()

	genFileComments[*genFileProto.Package+"."+serviceName] = serviceComment

	// Добавим сам Entity
	// удалим опцию спецификации
	entityPrefDesc.Options().ProtoReflect().Clear(apiSpecOpt.desc)
	entityMessageProtodesc := protodesc.ToDescriptorProto(entityPrefDesc)

	// Если сервис уже есть в генерации, добавим методы к нему
	// Если нет, создадим новый
	var genServiceProto *descriptorpb.ServiceDescriptorProto
	genServiceProto = &descriptorpb.ServiceDescriptorProto{
		Name: proto.String(serviceName),
	}
	for i := 0; i < len(genFileProto.Service); i++ {
		if *genFileProto.Service[i].Name == serviceName {
			genServiceProto = genFileProto.Service[i]
			// Удалим из списка, он будет добавлен после генерации
			genFileProto.Service = append(genFileProto.Service[:i], genFileProto.Service[i+1:]...)
			i = i - 1
		}
	}
	// в method_set описания сущности требуемые методы
	var requiredMethods map[string]Field
	if val, ok := entitySourceServiceFieldMap["method_set"]; ok {
		requiredMethods = getFieldMap(val.val.Message())
	}
	// В шаблоне спецификации шаблоны методов
	var tmplMethods map[string]pref.Message = make(map[string]pref.Message)
	methodTmplList := getFieldMap(tmplServiceFieldMap["method_set"].val.Message())["method_tmpl"].val.List()
	for i := range methodTmplList.Len() {
		tmplMethods[getFieldMap(methodTmplList.Get(i).Message())["spec_field_name"].val.String()] = methodTmplList.Get(i).Message()
	}

	// Добавим методы
	for _, method := range requiredMethods {
		if method.desc.IsList() {
			for j := range method.val.List().Len() {
				err := genMethod(
					tmplMethods[string(method.desc.Name())],
					method.val.List().Get(j).Message(),
					serviceKeyFieldsDefinition,
					genFileProto,
					entityPrefDesc,
					entityMessageProtodesc,
					genFileComments,
					addProtoRoot,
					httpRoot,
					googleApiAnnotationsPrefDesc,
					genServiceProto,
					serviceName,
				)
				if err != nil {
					return err
				}
			}
		} else {
			err := genMethod(
				tmplMethods[string(method.desc.Name())],
				method.val.Message(),
				serviceKeyFieldsDefinition,
				genFileProto,
				entityPrefDesc,
				entityMessageProtodesc,
				genFileComments,
				addProtoRoot,
				httpRoot,
				googleApiAnnotationsPrefDesc,
				genServiceProto,
				serviceName,
			)
			if err != nil {
				return err
			}
		}
	}
	genFileProto.Service = append(genFileProto.Service, genServiceProto)

	genFileProto.MessageType = append(genFileProto.MessageType, entityMessageProtodesc)

	return nil
}

func ToCamelCase(s string, divider string, joinW string) string {
	words := strings.Split(s, divider)
	for i := range words {
		words[i] = strings.ToUpper(string(words[i][0])) + strings.ToLower(words[i][1:])
	}

	return strings.Join(words, joinW)
}

func ToSnakeCase(s string, divider string) string {
	return strings.Replace(s, divider, "_", -1)
}

func printerSort(a, b protoprint.Element) bool {
	if a.Kind() == protoprint.KindService {
		if b.Kind() == protoprint.KindMessage {
			return true
		}
		if b.Kind() == protoprint.KindEnum {
			return true
		}
	}
	if a.Kind() == protoprint.KindMessage {
		if b.Kind() == protoprint.KindField {
			return true
		}
	}
	if a.Kind() == protoprint.KindEnum {
		if b.Kind() == protoprint.KindField {
			return true
		}
	}
	if a.Kind() == protoprint.KindField && b.Kind() == protoprint.KindField {
		return a.Number() < b.Number()
	}
	if a.Kind() == protoprint.KindEnumValue && b.Kind() == protoprint.KindEnumValue {
		return a.Number() < b.Number()
	}
	if a.Kind() == b.Kind() {
		return a.Name() < b.Name()
	}

	return false
}

func BuildEntityFeatures(entityFilePath string, importPaths []string) map[string]string {
	var protoFileNames []string
	m := make(map[string]string)
	err1 := filepath.Walk(entityFilePath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			split := strings.Split(path, ".")
			if !info.IsDir() && split[len(split)-1] == "proto" {
				protoFileNames = append(protoFileNames, strings.Replace(path, "\\", "/", -1))
			}
			return nil
		})
	if err1 != nil {
		log.Println(err1)
	}

	var warningErrorsWithPos []reporter.ErrorWithPos
	rep := reporter.NewReporter(
		func(errorWithPos reporter.ErrorWithPos) error {
			fmt.Println("Protocompile Error: ", errorWithPos)
			return errorWithPos
		},
		func(errorWithPos reporter.ErrorWithPos) {
			fmt.Println("Protocompile Warning:", errorWithPos)
			warningErrorsWithPos = append(warningErrorsWithPos, errorWithPos)
		},
	)

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: importPaths,
		}),
		Reporter:       rep,
		SourceInfoMode: protocompile.SourceInfoMode(1),
		RetainASTs:     true,
	}

	sourceFilePrefDesc, err := compiler.Compile(context.Background(), protoFileNames...)
	if err != nil {
		panic(err)
	}

	googleApiAnnotationsFiles, err := compiler.Compile(context.Background(), "google/api/annotations.proto")
	if err != nil {
		panic(err)
	}

	var filesJhumpDesc map[string]*desc.FileDescriptor = make(map[string]*desc.FileDescriptor)
	var genFileComments map[string]string = make(map[string]string)

	for k := range sourceFilePrefDesc {
		entityMsgApiSpecOpt := make(map[pref.MessageDescriptor]Field)
		entityMsgAmlSpecOpt := make(map[pref.MessageDescriptor]Field)
		// Соберем в массив сущности, на которые будем применять фичи
		for i := range sourceFilePrefDesc[k].Messages().Len() {
			msgDesc := sourceFilePrefDesc[k].Messages().Get(i)
			// смотрим какие именно заявлены фичи сущности
			// ищем опции entity.feature
			// TODO: рекурсивно смотреть во вложенные Messages
			for _, msgOpt := range getFieldMap(msgDesc.Options().ProtoReflect()) {
				// если есть опция на поле опции entity.feature.api.specification, считаем, что в поле описание спецификации АПИ
				msgOptOpt := getFieldMap(msgOpt.desc.Options().ProtoReflect())
				if val, ok := msgOptOpt["specification"]; ok {
					if val.fullName == "entity.feature.api.specification" {
						entityMsgApiSpecOpt[msgDesc] = msgOpt
					}
				}
				// если есть entity.feature.aml_specification
				if msgOpt.fullName == "entity.feature.aml.specification" {
					entityMsgAmlSpecOpt[msgDesc] = msgOpt

				}
			}
		}
		// Если есть сущности с entity.feature.aml_specification,обработаем
		if len(entityMsgAmlSpecOpt) > 0 {
			for entityPrefDesc, amlSpecOpt := range entityMsgAmlSpecOpt {
				fmt.Println(entityPrefDesc.FullName(), amlSpecOpt, ": entity.feature.aml_specification not implemented")
			}
		}
		// Если есть сущности с specification, генерируем файл АПИ спецификации
		if len(entityMsgApiSpecOpt) > 0 {
			// создаем файл для генерации
			sourceFileDescriptorProto := protodesc.ToFileDescriptorProto(sourceFilePrefDesc[k])
			sourceFileJhumpDesc := getProtoJ(compiler, sourceFileDescriptorProto, filesJhumpDesc)
			addProtoRoot := make(map[string]*descriptorpb.DescriptorProto)
			genFileProto := &descriptorpb.FileDescriptorProto{
				Syntax:     proto.String("proto3"),
				Name:       proto.String(string(sourceFilePrefDesc[k].FullName())),
				Package:    proto.String(string(sourceFilePrefDesc[k].Package())),
				Dependency: sourceFileDescriptorProto.Dependency,
				Options:    sourceFileDescriptorProto.Options,
			}
			// Типы без фичей specification добавим в генерацию
			for i := range sourceFilePrefDesc[k].Messages().Len() {
				if _, ok := entityMsgApiSpecOpt[sourceFilePrefDesc[k].Messages().Get(i)]; !ok {
					genFileProto.MessageType = append(genFileProto.MessageType, protodesc.ToDescriptorProto(sourceFilePrefDesc[k].Messages().Get(i)))
				}
			}

			// Существующие сервисы добавим в генерацию
			// Добавим комменты
			for i := range sourceFilePrefDesc[k].Services().Len() {
				genFileProto.Service = append(genFileProto.Service, protodesc.ToServiceDescriptorProto(sourceFilePrefDesc[k].Services().Get(i)))
				leadingComments := sourceFileJhumpDesc.FindService(string(sourceFilePrefDesc[k].Services().Get(i).FullName())).GetSourceInfo().LeadingComments
				if leadingComments != nil {
					genFileComments[string(sourceFilePrefDesc[k].Services().Get(i).FullName())] = *leadingComments
				}

				for j := range sourceFilePrefDesc[k].Services().Get(i).Methods().Len() {
					leadingComments := sourceFileJhumpDesc.
						FindService(string(sourceFilePrefDesc[k].Services().Get(i).FullName())).
						FindMethodByName(string(sourceFilePrefDesc[k].Services().Get(i).Methods().Get(j).Name())).
						GetSourceInfo().LeadingComments
					if leadingComments != nil {
						genFileComments[string(sourceFilePrefDesc[k].Services().Get(i).Methods().Get(j).FullName())] =
							*leadingComments
					}
				}
			}

			// Добавим Enum
			for i := range sourceFilePrefDesc[k].Enums().Len() {
				genFileProto.EnumType = append(genFileProto.EnumType, protodesc.ToEnumDescriptorProto(sourceFilePrefDesc[k].Enums().Get(i)))
				leadingComments := sourceFileJhumpDesc.FindEnum(string(sourceFilePrefDesc[k].Enums().Get(i).FullName())).GetSourceInfo().LeadingComments
				if leadingComments != nil {
					genFileComments[string(sourceFilePrefDesc[k].Enums().Get(i).FullName())] = *leadingComments
				}
				for j := range sourceFilePrefDesc[k].Enums().Get(i).Values().Len() {
					enumValue := sourceFileJhumpDesc.
						FindEnum(string(sourceFilePrefDesc[k].Enums().Get(i).FullName())).
						FindValueByName(string(sourceFilePrefDesc[k].Enums().Get(i).Values().Get(j).Name()))
					leadingComments := enumValue.GetSourceInfo().LeadingComments
					if leadingComments != nil {
						genFileComments[enumValue.GetFullyQualifiedName()] = *leadingComments
					}
				}
			}

			// Добавим опции файла
			for _, v := range getFieldMap(sourceFilePrefDesc[k].Options().ProtoReflect()) {
				if v.desc.Kind() == pref.StringKind {
					genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageName}", string(sourceFilePrefDesc[k].Package()), -1)))
					genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameDotCamelCase}", ToCamelCase(string(sourceFilePrefDesc[k].Package()), ".", "."), -1)))
					genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameSnakeCase}", ToSnakeCase(string(sourceFilePrefDesc[k].Package()), "."), -1)))
					genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameCamelCase}", ToCamelCase(string(sourceFilePrefDesc[k].Package()), ".", ""), -1)))
					genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameUpperCase}", strings.ToUpper(strings.Replace(string(sourceFilePrefDesc[k].Package()), ".", "", -1)), -1)))
					genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameDoubleSlashCamelCase}", ToCamelCase(string(sourceFilePrefDesc[k].Package()), ".", "\\"), -1)))
					genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameDoubleColonCamelCase}", ToCamelCase(string(sourceFilePrefDesc[k].Package()), ".", "::"), -1)))
				}
			}

			// Заполним комменты по всем типам файла
			for i := range sourceFileJhumpDesc.GetMessageTypes() {
				getMessageComments(sourceFileJhumpDesc.GetMessageTypes()[i], genFileComments)
			}

			// Заполним комменты по всем сервисам файла
			for i := range sourceFileJhumpDesc.GetServices() {
				if sourceFileJhumpDesc.GetServices()[i].GetSourceInfo().LeadingComments != nil {
					genFileComments[sourceFileJhumpDesc.GetServices()[i].GetFullyQualifiedName()] = *sourceFileJhumpDesc.GetServices()[i].GetSourceInfo().LeadingComments
				}
			}

			// Обработаем сущности
			for entityPrefDesc, apiSpecOpt := range entityMsgApiSpecOpt {
				// Добавим комментарий всего файла
				if val, ok := genFileComments[sourceFileJhumpDesc.GetName()]; ok {
					genFileComments[sourceFileJhumpDesc.GetName()] = val + "\n// Спецификация АПИ управления сущностью " + string(entityPrefDesc.Name())
				} else {
					genFileComments[sourceFileJhumpDesc.GetName()] = "// Спецификация АПИ управления сущностью " + string(entityPrefDesc.Name())
				}

				// Обработаем cущность
				err := genEntityApiSpec(apiSpecOpt,
					entityPrefDesc,
					genFileProto,
					googleApiAnnotationsFiles[0],
					genFileComments,
					addProtoRoot)
				if err != nil {
					panic(err)
				}
			}

			// уберем опции entity.feature с полей, они не нужны в генерации
			for i := range genFileProto.MessageType {
				for j := range genFileProto.MessageType[i].Field {
					m := getFieldMap(genFileProto.MessageType[i].Field[j].Options.ProtoReflect())
					for _, val := range m {
						if strings.Contains(string(val.fullName), "entity.feature.") {
							genFileProto.MessageType[i].Field[j].Options.ProtoReflect().Clear(val.desc)
						}
					}
				}
			}

			// Напечатаем генерацию
			// Добавим типы, которые пришли с шаблонами, но не внутри шаблона
			for _, v := range addProtoRoot {
				for i := 0; i < len(genFileProto.MessageType); i++ {
					if *genFileProto.MessageType[i].Name == *v.Name {
						fmt.Println("Тип " + *v.Name + " уже присутствует в генерации, он будет заменен")
						genFileProto.MessageType = append(genFileProto.MessageType[:i], genFileProto.MessageType[i+1:]...)
						i = i - 1
					}
				}
				genFileProto.MessageType = append(genFileProto.MessageType, v)
			}
			// Уберем опции entity.feature
			var deps map[string]string = make(map[string]string)
			for _, v := range entityMsgApiSpecOpt {
				// убираем импорт entity-tmpl/entity-feature.proto
				for i := range genFileProto.Dependency {
					if genFileProto.Dependency[i] != string(v.desc.ParentFile().FullName())+".proto" &&
						genFileProto.Dependency[i] != "entity.feature.proto" {
						deps[genFileProto.Dependency[i]] = genFileProto.Dependency[i]
					}
				}
			}
			var depsArr []string
			for k := range deps {
				depsArr = append(depsArr, k)
			}
			genFileProto.Dependency = depsArr

			var dpj []*desc.FileDescriptor
			for _, v := range filesJhumpDesc {
				dpj = append(dpj, v)
			}

			genFileJhumpDesc, err := desc.CreateFileDescriptor(genFileProto, dpj...)
			if err != nil {
				panic(err)
			}

			genFileBuilder, err := builder.FromFile(genFileJhumpDesc)
			if err != nil {
				panic(err)
			}

			// Добавим комменты в генерацию
			for i := range genFileJhumpDesc.GetServices() {
				if val, ok := genFileComments[genFileJhumpDesc.GetServices()[i].GetFullyQualifiedName()]; ok {
					genFileBuilder.GetService(genFileJhumpDesc.GetServices()[i].GetName()).SetComments(builder.Comments{LeadingComment: val})
				}
				for j := range genFileJhumpDesc.GetServices()[i].GetMethods() {
					if val, ok := genFileComments[genFileJhumpDesc.GetServices()[i].GetMethods()[j].GetFullyQualifiedName()]; ok {
						genFileBuilder.GetService(genFileJhumpDesc.GetServices()[i].GetName()).GetMethod(genFileJhumpDesc.GetServices()[i].GetMethods()[j].GetName()).SetComments(builder.Comments{LeadingComment: val})
					}
				}
			}
			for i := range genFileJhumpDesc.GetMessageTypes() {
				fillMessageComments(genFileJhumpDesc.GetMessageTypes()[i], genFileBuilder.GetMessage(genFileJhumpDesc.GetMessageTypes()[i].GetName()), genFileComments)
			}
			for i := range genFileJhumpDesc.GetEnumTypes() {
				if val, ok := genFileComments[genFileJhumpDesc.GetEnumTypes()[i].GetFullyQualifiedName()]; ok {
					genFileBuilder.GetEnum(genFileJhumpDesc.GetEnumTypes()[i].GetName()).SetComments(builder.Comments{LeadingComment: val})
				}
				for j := range genFileJhumpDesc.GetEnumTypes()[i].GetValues() {
					fmt.Println("leadingComments", genFileJhumpDesc.GetEnumTypes()[i].GetValues()[j].GetFullyQualifiedName())
					if val, ok := genFileComments[genFileJhumpDesc.GetEnumTypes()[i].GetValues()[j].GetFullyQualifiedName()]; ok {
						fmt.Println("leadingComments", val)
						genFileBuilder.GetEnum(genFileJhumpDesc.GetEnumTypes()[i].GetName()).
							GetValue(genFileJhumpDesc.GetEnumTypes()[i].GetValues()[j].GetName()).
							SetComments(builder.Comments{LeadingComment: val})
					}
				}
			}

			genFileDesc, err := genFileBuilder.Build()
			if err != nil {
				panic(err)
			}

			p := new(protoprint.Printer)
			p.CustomSortFunction = printerSort
			p.SortElements = true
			p.Compact = true
			p.ForceFullyQualifiedNames = false
			p.Indent = "    "
			protoStr, err := p.PrintProtoToString(genFileDesc)
			if err != nil {
				panic(err)
			}
			// TODO: Добавить параметры генерации в коммент файла
			protoStr = genFileComments[sourceFileJhumpDesc.GetName()] + "\n\n" + protoStr
			// fmt.Println("print Proto", protoStr)
			m[sourceFileJhumpDesc.GetFile().GetName()] = protoStr
		}
	}

	fmt.Println("Generation end")
	return m
}

func main() {
	protoPath := os.Getenv("PROTO_PATH")
	protoOutPath := os.Getenv("PROTO_OUT_PATH")
	m := BuildEntityFeatures(protoPath, []string{".", "proto_deps"})
	fmt.Println("PROTO_PATH", protoPath)
	fmt.Println("PROTO_OUT_PATH", protoOutPath)
	if protoOutPath != "" {
		for k, v := range m {
			os.MkdirAll(filepath.Dir(protoOutPath+k),0755)

			// permissions := os.FileMode(0644)

			// file, err := os.Create(protoOutPath + k)
			// if err != nil {
			// 	log.Fatalf("Error creating file: %v", err)
			// }
			// defer file.Close() // Ensure the file is closed

			// Write the string content (converted to a byte slice) to the file
			err := os.WriteFile(protoOutPath+k, []byte(v), 0644)
			if err != nil {
				log.Fatalf("Failed to write to file: %v", err)
			}
		}
	}
}
