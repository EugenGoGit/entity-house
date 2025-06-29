package main

import (
	"context"
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
	"github.com/bufbuild/protocompile/linker"
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
)

//         jhump файл дескриптор
//             нельзя собрать
//             нельзя менять поля
//             можно создать с descproto файл
//             можно сменять в descproto файл
//             нельзя поставить комменты, можно читать коммменты
//             опции не читаются
//             protoprint
//         jhump message дескриптор
//             нельзя собрать, только найти в jhump файл дескриптор
//             нельзя менять поля
//             нельзя поставить комменты, можно читать коммменты
//             опции не читаются
//         jhump файл builder
//             можно собрать
//             можно менять поля
//             можно создать с jhump файл дескриптор
//             можно сбилдить в jhump файл дескриптор
//             можно поставить комменты
//             опции читаются и меняются
//         jhump message builder
//             можно собрать
//             можно менять поля
//             можно создать с jhump файл дескриптор
//             можно сбилдить в jhump файл дескриптор
//             можно поставить комменты
//             опции читаются и меняются
//         protoreflect файл дескриптор
//             нельзя собрать
//             нельзя менять поля
//             можно создать с descproto файл
//             можно сменять в descproto файл
//             можно взять с jhump файл дескриптор
//             нельзя поставить комменты, нельзя читать коммменты
//             опции читаются, значения читаются
//         protoreflect message дескриптор
//             нельзя собрать ??? options.shallowCopy
//             нельзя менять поля
//             можно создать с descproto message
//             можно сменять в descproto message
//             можно взять с jhump message дескриптор
//             нельзя поставить комменты, нельзя читать коммменты
//             опции читаются, значения читаются
//         descproto файл
//             можно собрать
//             можно менять поля
//             можно создать с jhump файл дескриптор
//             можно сбилдить в jhump файл дескриптор
//             можно создать с protoreflect файл дескриптор
//             можно сбилдить в protoreflect файл дескриптор
//             нельзя поставить комменты
//             опции читаются и меняются
//         descproto message дескриптор
//             можно собрать
//             можно менять поля
//             можно взять с jhump message дескриптор
//             нельзя сбилдить в jhump message дескриптор
//             можно создать с protoreflect message дескриптор
//             можно сбилдить в protoreflect message дескриптор
//             нельзя поставить комменты
//             опции читаются и меняются

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

func createMessageDescByTemplate(templateParent *pref.Value,
	messageName string,
	messageFullName string,
	replaceTo *descriptorpb.DescriptorProto,
	replaceKeysToFields []FieldFullName,
	fileCommentsMap map[string]string,
	// TODO: fileJhumpDesc не используется
	fileJhumpDesc map[string]*desc.FileDescriptor,
	addMessageToProtoRoot map[string]pref.MessageDescriptor,
) *descriptorpb.DescriptorProto {
	var messagePrefDesc pref.MessageDescriptor
	var messageProtodesc *descriptorpb.DescriptorProto
	var templateTypeName string
	if templateParent.String() != "<nil>" {
		// Берем шаблон из поля, поле там одно
		for _, v := range getFieldMap(templateParent.Message()) {
			messagePrefDesc = v.val.Message().Descriptor()
			templateTypeName = string(v.val.Message().Descriptor().FullName())
			break
		}
		messageProtodesc = protodesc.ToDescriptorProto(messagePrefDesc)
		messageProtodesc.Name = &messageName
		changeFieldTypeNameNested(messageProtodesc,
			messagePrefDesc,
			".entity.feature.api.options.EntityDescriptor",
			".entity.feature.api.options.EntityKeyDescriptor",
			*replaceTo.Name,
			replaceKeysToFields,
			templateTypeName,
			messageName,
			messageFullName,
			fileCommentsMap,
			addMessageToProtoRoot,
		)
	} else {
		// Если template не задан используем пустой message
		messageProtodesc = &descriptorpb.DescriptorProto{
			Name: &messageName,
		}
	}
	return messageProtodesc
}

func changeFieldTypeNameNested(messageDescProto *descriptorpb.DescriptorProto,
	messagePrefDesc pref.MessageDescriptor,
	replaceFromType string,
	replaceKeysFromType string,
	replaceToType string,
	replaceKeysToFields []FieldFullName,
	parentNameFromType string,
	parentNameToType string,
	parentFullNameToType string,
	fileCommentsMap map[string]string,
	addMessageToProtoRoot map[string]pref.MessageDescriptor,
) {
	var i int32 = 0
	for len(messageDescProto.Field) > int(i) {
		if *messageDescProto.Field[i].Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			if *messageDescProto.Field[i].TypeName == replaceFromType {
				// Меняем тип поля на тип сущности и добавляем коммент от сущности
				messageDescProto.Field[i].TypeName = &replaceToType
				if val, ok := fileCommentsMap[replaceToType]; ok {
					fileCommentsMap[parentFullNameToType+"."+messageDescProto.Field[i].GetName()] = val
				}
			} else {
				if *messageDescProto.Field[i].TypeName == replaceKeysFromType {
					// Вставляем ключи с сущности и комменты с ключей с сущности
					replaceKeysToFields[0].fieldDescProto.Number = messageDescProto.Field[i].Number
					opts := getFieldMap(messageDescProto.Field[i].Options.ProtoReflect())
					for _, v := range opts {
						replaceKeysToFields[0].fieldDescProto.GetOptions().ProtoReflect().NewField(v.desc)
						replaceKeysToFields[0].fieldDescProto.GetOptions().ProtoReflect().Set(v.desc, v.val)
					}

					messageDescProto.Field[i] = replaceKeysToFields[0].fieldDescProto
					fileCommentsMap[string(parentFullNameToType)+"."+messageDescProto.Field[i].GetName()] = fileCommentsMap[replaceKeysToFields[0].fullName]

					for j := 1; j < len(replaceKeysToFields); j++ {
						n := *replaceKeysToFields[j-1].fieldDescProto.Number + int32(1)
						replaceKeysToFields[j].fieldDescProto.Number = &n
						for _, v := range opts {
							replaceKeysToFields[j].fieldDescProto.GetOptions().ProtoReflect().NewField(v.desc)
							replaceKeysToFields[j].fieldDescProto.GetOptions().ProtoReflect().Set(v.desc, v.val)
						}
						// replaceKeysToFields[j].fieldDescProto.Options = *opts
						if len(messageDescProto.Field) == int(i+1) {
							messageDescProto.Field = append(messageDescProto.Field, replaceKeysToFields[j].fieldDescProto)
						} else {
							messageDescProto.Field = append(messageDescProto.Field[:i+1], messageDescProto.Field[i:]...) // index < len(a)
							messageDescProto.Field[i+1] = replaceKeysToFields[j].fieldDescProto
						}
						i = i + 1
						fileCommentsMap[string(parentFullNameToType)+"."+messageDescProto.Field[i].GetName()] = fileCommentsMap[replaceKeysToFields[j].fullName]
					}
					// Проставим номера полей на случай пересечения номеров
					for j := range messageDescProto.Field {
						n := int32(j + 1)
						if n > *messageDescProto.Field[j].Number {
							messageDescProto.Field[j].Number = &n
						}
					}

				} else {
					// Поле не заменяем, заменяем тип на тип поля в результате применения шаблона
					// ставим комменты из опции field_comments поля в шаблоне, убираем опцию
					if val, ok := getFieldMap(messageDescProto.Field[i].Options.ProtoReflect())["field_comments"]; ok {
						fileCommentsMap[parentFullNameToType+"."+messageDescProto.Field[i].GetName()] = val.val.String()
						messageDescProto.Field[i].GetOptions().ProtoReflect().Clear(val.desc)
					}
					s := strings.Replace(*messageDescProto.Field[i].TypeName, "."+parentNameFromType+".", parentNameToType+".", -1)

					fmt.Println("messageDescProto.Field[i].TypeName",
						*messageDescProto.Field[i].TypeName,
						s,
						messagePrefDesc.Fields().Get(int(i)).Message().FullName(),
						parentNameFromType,
						parentNameToType,
						parentFullNameToType)
					// Если замены не произошло, значит тип не в Nested типах и нужно добавить его в генерацию отдельно от результата шаблона
					if s == *messageDescProto.Field[i].TypeName {
						
														changeFieldTypeNameNested(messageDescProto.NestedType[i],
			messagePrefDesc.Messages().ByName(pref.Name(*messageDescProto.NestedType[i].Name)),
			replaceFromType,
			replaceKeysFromType,
			replaceToType,
			replaceKeysToFields,
			parentNameFromType,
			parentNameToType,
			parentFullNameToType+"."+messageDescProto.NestedType[i].GetName(),
			fileCommentsMap,
			addMessageToProtoRoot,
		)
						addMessageToProtoRoot[string(messagePrefDesc.Fields().Get(int(i)).Message().Name())] = messagePrefDesc.Fields().Get(int(i)).Message()
						s = string(messagePrefDesc.Fields().Get(int(i)).Message().Name())
								changeFieldTypeNameNested(messageDescProto.NestedType[i],
			messagePrefDesc.Messages().ByName(pref.Name(*messageDescProto.NestedType[i].Name)),
			replaceFromType,
			replaceKeysFromType,
			replaceToType,
			replaceKeysToFields,
			parentNameFromType,
			parentNameToType,
			parentFullNameToType+"."+messageDescProto.NestedType[i].GetName(),
			fileCommentsMap,
			addMessageToProtoRoot,
		)
					}
					messageDescProto.Field[i].TypeName = &s
				}
			}
		} else {
			// Поле не заменяем, скалярный тип оставляем, ставим комменты из опции field_comments поля в шаблоне, убираем опцию
			if val, ok := getFieldMap(messageDescProto.Field[i].Options.ProtoReflect())["field_comments"]; ok {
				fileCommentsMap[parentFullNameToType+"."+messageDescProto.Field[i].GetName()] = val.val.String()
				messageDescProto.Field[i].GetOptions().ProtoReflect().Clear(val.desc)
			}
			if val, ok := fileCommentsMap[parentNameFromType+"."+messageDescProto.Field[i].GetName()]; ok {
				fileCommentsMap[parentFullNameToType+"."+messageDescProto.Field[i].GetName()] = val
			}
		}
		i = i + 1
	}

	// Ставим коммент из опции message_comments и уберем эту опцию
	// поменяем тип на нового парента
	// рекурсивно меняем вложенные конструкции
	for i := range messageDescProto.NestedType {
		if val, ok := getFieldMap(messageDescProto.NestedType[i].GetOptions().ProtoReflect())["message_comments"]; ok {
			fileCommentsMap[parentFullNameToType+"."+messageDescProto.NestedType[i].GetName()] = val.val.String()
			messageDescProto.NestedType[i].GetOptions().ProtoReflect().Clear(val.desc)
		}
		s := strings.Replace(*messageDescProto.NestedType[i].Name, parentNameFromType, parentFullNameToType, -1)
		messageDescProto.NestedType[i].Name = &s

		changeFieldTypeNameNested(messageDescProto.NestedType[i],
			messagePrefDesc.Messages().ByName(pref.Name(*messageDescProto.NestedType[i].Name)),
			replaceFromType,
			replaceKeysFromType,
			replaceToType,
			replaceKeysToFields,
			parentNameFromType,
			parentNameToType,
			parentFullNameToType+"."+messageDescProto.NestedType[i].GetName(),
			fileCommentsMap,
			addMessageToProtoRoot,
		)
	}

	// Ставим коммент из опции oneof_comments и уберем эту опцию
	for i := range messageDescProto.GetOneofDecl() {
		if val, ok := getFieldMap(messageDescProto.GetOneofDecl()[i].GetOptions().ProtoReflect())["oneof_comments"]; ok {
			fileCommentsMap[string(parentFullNameToType)+"."+messageDescProto.GetOneofDecl()[i].GetName()] = val.val.String()
			messageDescProto.GetOneofDecl()[i].GetOptions().ProtoReflect().Clear(val.desc)
		}
	}
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

// TODO: remove fullName
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

// TODO: убрать prefixName
func getMessageComments(messJhumpDesc *desc.MessageDescriptor, prefixName string, commentsMap map[string]string) {
	//commentsMap[prefixName+"."+string(messJhumpDesc.GetName())] = *messJhumpDesc.GetSourceInfo().LeadingComments
	fmt.Println("messJhumpDesc.GetFullyQualifiedName()", messJhumpDesc.GetFullyQualifiedName(), prefixName, string(messJhumpDesc.GetName()))
	commentsMap[messJhumpDesc.GetFullyQualifiedName()] = *messJhumpDesc.GetSourceInfo().LeadingComments
	for i := range messJhumpDesc.GetNestedMessageTypes() {
		getMessageComments(messJhumpDesc.GetNestedMessageTypes()[i],
			//prefixName+"."+string(messJhumpDesc.GetName()),
			messJhumpDesc.GetFullyQualifiedName(),
			commentsMap,
		)
	}
	for i := range messJhumpDesc.GetFields() {
		if messJhumpDesc.GetFields()[i].GetSourceInfo().LeadingComments != nil {
			//commentsMap[prefixName+"."+string(messJhumpDesc.GetName())+"."+messJhumpDesc.GetFields()[i].GetName()] = *messJhumpDesc.GetFields()[i].GetSourceInfo().LeadingComments
			commentsMap[messJhumpDesc.GetFullyQualifiedName()+"."+messJhumpDesc.GetFields()[i].GetName()] = *messJhumpDesc.GetFields()[i].GetSourceInfo().LeadingComments

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
}

func genEntityApiSpec(apiSpecOpt Field,
	entityPrefDesc pref.MessageDescriptor,
	sourceFileJhumpDesc *desc.FileDescriptor,
	genFileProto *descriptorpb.FileDescriptorProto,
	googleApiAnnotationsPrefDesc pref.FileDescriptor,
	genFileComments map[string]string,
	filesJhumpDesc map[string]*desc.FileDescriptor,
	addProtoRoot map[string]pref.MessageDescriptor,
) {

	// берем поле фичи, в котором описан сервис
	// считаем, что поле там только одно
	var serviceOptVal pref.Message
	for _, value := range getFieldMap(apiSpecOpt.val.Message()) {
		serviceOptVal = value.val.Message()
		break
	}
	// Создаем сервис
	// в опциях шаблон имени
	serviceOptsMap := getFieldMap(serviceOptVal.Descriptor().Options().ProtoReflect())
	serviceName := strings.Replace(serviceOptsMap["name_template"].val.String(), "{EntityName}", string(entityPrefDesc.Name()), -1)

	// в полях сервиса требуемые методы в method_set
	serviceFieldsMap := getFieldMap(serviceOptVal)
	methodsDefMap := getFieldMap(serviceFieldsMap["method_set"].val.Message())
	// Корневой httpPath в http_root, он же - признак добавления опций http
	httpRoot := serviceFieldsMap["http_root"].val.String()
	// Определение уникальных полей сервиса, будет использовано, если не переопределено на уровне метода
	serviceKeyFieldsDefinition := serviceFieldsMap["key_fields_definition"].val

	genServiceProto := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String(serviceName),
	}
	genFileComments[*genFileProto.Package+"."+serviceName] = "Сервис управления сущностью " + string(entityPrefDesc.Name())

	// Найдем комменты Entity
	var num int
	for i := range sourceFileJhumpDesc.GetMessageTypes() {
		if sourceFileJhumpDesc.GetMessageTypes()[i].GetName() == string(entityPrefDesc.Name()) {
			num = i
		}
	}
	getMessageComments(sourceFileJhumpDesc.GetMessageTypes()[num], *genFileProto.Package, genFileComments)

	// Добавим сам Entity
	entityMessageProtodesc := protodesc.ToDescriptorProto(entityPrefDesc)
	// Добавим методы
	for _, methodDef := range methodsDefMap {
		// в полях заданы параметры метода
		methodFieldMap := getFieldMap(methodDef.val.Message())
		// переопределяем Определение уникальных полей сервиса
		keyFieldsDefinition := serviceKeyFieldsDefinition
		if val, ok := methodFieldMap["key_fields_definition"]; ok {
			keyFieldsDefinition = val.val
		}
		// в опциях параметры генерации метода
		methodOptsMap := getFieldMap(methodDef.val.Message().Descriptor().Options().ProtoReflect())
		// в полях entity.api.method_component_template_set шаблоны компонент метода
		methodTemplatesMap := getFieldMap(methodOptsMap["method_component_template_set"].val.Message())
		// шаблон имени в name_template
		methodName := strings.Replace(methodTemplatesMap["name_template"].val.String(), "{EntityName}", string(entityPrefDesc.Name()), -1)
		// methodFullName := strings.Replace(methodTemplatesMap["name_template"].val.String(), "{EntityName}", string(msgDesc.FullName()), -1)
		// шаблон имени запроса в request_name
		requestName := strings.Replace(methodTemplatesMap["request_name"].val.String(), "{EntityName}", string(entityPrefDesc.Name()), -1)
		requestFullName := *genFileProto.Package + "." + requestName
		// шаблон имени ответа в response_name
		responseName := strings.Replace(methodTemplatesMap["response_name"].val.String(), "{EntityName}", string(entityPrefDesc.Name()), -1)
		responseFullName := *genFileProto.Package + "." + responseName
		// определяем список ключевых полей для метода
		keyFieldList := getKeyFields(&keyFieldsDefinition, entityPrefDesc)
		fmt.Println("keyFieldList", keyFieldList)
		// шаблон запроса в request_template
		// там только одно поле
		tmpl := methodTemplatesMap["request_template"]

		if tmpl.val.String() != "<nil>" {
			fmt.Println("requestFullName", tmpl.val.Message().Descriptor().ParentFile().FullName().Name())
		}
		requestMessageProtodesc := createMessageDescByTemplate(
			&tmpl.val,
			requestName,
			requestFullName,
			entityMessageProtodesc,
			keyFieldList,
			genFileComments,
			filesJhumpDesc,
			addProtoRoot,
		)

		tmpl = methodTemplatesMap["response_template"]
		responseMessageProtodesc := createMessageDescByTemplate(
			&tmpl.val,
			responseName,
			responseFullName,
			entityMessageProtodesc,
			keyFieldList,
			genFileComments,
			filesJhumpDesc,
			addProtoRoot,
		)

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
		if val, ok := methodOptsMap["server_streaming"]; ok {
			if val.val.Bool() {
				b := true
				genMethodProto.ServerStreaming = &b
			} else {
				b := false
				genMethodProto.ServerStreaming = &b
			}
		} else {
			b := false
			genMethodProto.ServerStreaming = &b
		}
		// в поле client_streaming признак ответа в потоке
		if val, ok := methodOptsMap["client_streaming"]; ok {
			if val.val.Bool() {
				b := true
				genMethodProto.ClientStreaming = &b
			} else {
				b := false
				genMethodProto.ClientStreaming = &b
			}
		} else {
			b := false
			genMethodProto.ClientStreaming = &b
		}
		// в полях entity.api.http_rule http опции google.api.HttpRule
		// если заданы http_rule и httpRoot, то добавляем опции google.api.http для этого метода
		// с заменой по шаблону
		if val, ok := methodOptsMap["http_rule"]; ok {
			if httpRoot != "" && httpRoot != "<nil>" {
				var keyFieldPath string
				for i := range keyFieldList {
					keyFieldPath = keyFieldPath + *keyFieldList[i].fieldDescProto.Name + "/{" + *keyFieldList[i].fieldDescProto.Name + "}"
					if i != len(keyFieldList)-1 {
						keyFieldPath = keyFieldPath + "/"
					}
				}
				methodHttpRuleMap := getFieldMap(val.val.Message())
				var valOpt string = "{ "

				fdHttp := googleApiAnnotationsPrefDesc.Extensions().ByName("http")
				fdHttpV := dynamicpb.NewMessage(fdHttp.Message())
				for k, v := range methodHttpRuleMap {
					fd := fdHttp.Message().Fields().ByName(pref.Name(k))
					fdHttpV.Set(fd, pref.ValueOf(strings.Replace(strings.Replace(v.val.String(), "{HttpRoot}", httpRoot, -1), "{KeyFields}", keyFieldPath, -1)))
					valOpt = valOpt + k + ": \"" +
						strings.Replace(strings.Replace(v.val.String(), "{HttpRoot}", httpRoot, -1), "{KeyFields}", keyFieldPath, -1) +
						"\", "
				}
				genMethodProto.Options.ProtoReflect().Set(fdHttp, pref.ValueOf(fdHttpV))

			}

		}

		// в поле entity.rules.key_field_behavior поведение полей ключа, требуемое для данного метода
		printEnumEl(methodOptsMap["key_field_behavior"])

		// Добавляем метод с комментарием
		// Добавляем комментарии к запросу и ответу метода
		genServiceProto.Method = append(genServiceProto.Method, genMethodProto)
		methodComment := "Метод " + methodName
		if val, ok := methodTemplatesMap["leading_comment"]; ok {
			methodComment = strings.Replace(val.val.String(), "{EntityName}", string(entityPrefDesc.Name()), -1)
		}
		requestComment := "Запрос метода " + methodName
		responseComment := "Ответ на запрос метода " + methodName

		if val, ok := methodFieldMap["custom_comments"]; ok {
			customCommentMap := getFieldMap(val.val.Message())
			if val, ok := customCommentMap["leading_comment"]; ok {
				methodComment = val.val.String()
			}
			if val, ok := customCommentMap["additional_leading_comment"]; ok {
				methodComment = methodComment + ".\n" + val.val.String()
			}
			if val, ok := customCommentMap["request_leading_comment"]; ok {
				requestComment = val.val.String()
			}
			if val, ok := customCommentMap["response_leading_comment"]; ok {
				responseComment = val.val.String()
			}
		}
		genFileComments[*genFileProto.Package+"."+serviceName+"."+methodName] = methodComment
		genFileComments[*genFileProto.Package+"."+requestName] = requestComment
		genFileComments[*genFileProto.Package+"."+responseName] = responseComment

	}

	genFileProto.Service = append(genFileProto.Service, genServiceProto)

	genFileProto.MessageType = append(genFileProto.MessageType, entityMessageProtodesc)
}

func getOrCreateFileDescriptorProto(fileName string, filePrefDesc pref.FileDescriptor, fileDescriptorProtoMap map[string]*descriptorpb.FileDescriptorProto) *descriptorpb.FileDescriptorProto {
	if val, ok := fileDescriptorProtoMap[fileName]; ok {
		return val
	}
	fileDescriptorProtoMap[fileName] = protodesc.ToFileDescriptorProto(filePrefDesc)
	return fileDescriptorProtoMap[fileName]
}

func getOrCreateGenFileDescriptorProto(fileName string, fileDescriptorProtoMap map[string]*descriptorpb.FileDescriptorProto, dependency []string, options *descriptorpb.FileOptions, packageName string) *descriptorpb.FileDescriptorProto {
	if val, ok := fileDescriptorProtoMap[fileName]; ok {
		return val
	}
	// создаем файл для генерации
	genFileProto := &descriptorpb.FileDescriptorProto{
		Syntax:     proto.String("proto3"),
		Name:       proto.String(fileName),
		Package:    proto.String(packageName),
		Dependency: dependency,
		Options:    options,
	}
	fileDescriptorProtoMap[fileName] = genFileProto
	return genFileProto
}

func getOrCreateFileJhumpDesc(fileName string, compiler protocompile.Compiler, fileDescriptorProto *descriptorpb.FileDescriptorProto, filesJhumpDesc map[string]*desc.FileDescriptor) *desc.FileDescriptor {
	if val, ok := filesJhumpDesc[fileName]; ok {
		return val
	}
	filesJhumpDesc[fileName] = getProtoJ(compiler, fileDescriptorProto, filesJhumpDesc)
	return filesJhumpDesc[fileName]
}

func getOrCreateAddProtoRoot(fileName string, filesAddProtoRoot map[string]map[string]pref.MessageDescriptor) map[string]pref.MessageDescriptor {
	if val, ok := filesAddProtoRoot[fileName]; ok {
		return val
	}
	filesAddProtoRoot[fileName] = make(map[string]pref.MessageDescriptor)
	return filesAddProtoRoot[fileName]
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

func BuildEntityFeatures(entityFilePath string, importPaths []string) map[string]string {
	var protoFileNames []string
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

	fmt.Println("entityProtoFileNames", protoFileNames)

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

	var sourceFilesPrefDesc map[string]linker.File = make(map[string]linker.File)
	var filesJhumpDesc map[string]*desc.FileDescriptor = make(map[string]*desc.FileDescriptor)
	var sourceFilesDescProto map[string]*descriptorpb.FileDescriptorProto = make(map[string]*descriptorpb.FileDescriptorProto)
	var genFilesDescProto map[string]*descriptorpb.FileDescriptorProto = make(map[string]*descriptorpb.FileDescriptorProto)
	var filesAddProtoRoot map[string]map[string]pref.MessageDescriptor = make(map[string]map[string]pref.MessageDescriptor)
	var genFileComments map[string]string = make(map[string]string)
	for i := range protoFileNames {
		filePrefDesc, err := compiler.Compile(context.Background(), protoFileNames[i])
		if err != nil {
			panic(err)
		}
		sourceFilesPrefDesc[protoFileNames[i]] = filePrefDesc[0]
	}

	googleApiAnnotationsFiles, err := compiler.Compile(context.Background(), "google/api/annotations.proto")
	if err != nil {
		panic(err)
	}

	m := make(map[string]string)
	for sourceFileName, sourceFilePrefDesc := range sourceFilesPrefDesc {
		for i := range sourceFilePrefDesc.Messages().Len() {
			msgDesc := sourceFilePrefDesc.Messages().Get(i)
			// смотрим какие именно заявлены фичи сущности
			// ищем опции entity.feature
			msgOptsMap := getFieldMap(msgDesc.Options().ProtoReflect())
			for _, val := range msgOptsMap {
				if strings.Contains(string(val.fullName), "entity.feature.") {
					// если есть entity.feature.api_specification, считаем, что в поле api_specification описание сервиса
					// считаем, что опция Kind = message
					// считаем, что данный Message описывает сущность для которой нужен сервис
					if val.fullName == "entity.feature.api_specification" {
						sourceFileDescriptorProto := getOrCreateFileDescriptorProto(sourceFileName, sourceFilePrefDesc, sourceFilesDescProto)
						sourceFileJhumpDesc := getOrCreateFileJhumpDesc(sourceFileName, compiler, sourceFileDescriptorProto, filesJhumpDesc)
						addProtoRoot := getOrCreateAddProtoRoot(sourceFileName, filesAddProtoRoot)
						genFileProto := getOrCreateGenFileDescriptorProto(
							sourceFileName,
							genFilesDescProto,
							sourceFileDescriptorProto.Dependency,
							sourceFileDescriptorProto.Options,
							string(sourceFilePrefDesc.Package()),
						)
						// Возьмем комментарий с файла
						genFileComments[sourceFileJhumpDesc.GetName()] = "АПИ управления сущностью " + string(msgDesc.Name())
						// Добавим опции файла
						for k, v := range getFieldMap(sourceFilePrefDesc.Options().ProtoReflect()) {
							if v.desc.Kind() == pref.StringKind {
								// valStr := strings.Replace(v.val.String(), "{PackageName}", string(sourceFilePrefDesc.Package()), -1))
								// valStr = strings.Replace(v.val.String(), "{PackageNameDotCamelCase}", ToCamelCase(string(sourceFilePrefDesc.Package()), "."), -1))
								genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageName}", string(sourceFilePrefDesc.Package()), -1)))
								genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameDotCamelCase}", ToCamelCase(string(sourceFilePrefDesc.Package()), ".", "."), -1)))
								genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameSnakeCase}", ToSnakeCase(string(sourceFilePrefDesc.Package()), "."), -1)))
								genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameCamelCase}", ToCamelCase(string(sourceFilePrefDesc.Package()), ".", ""), -1)))
								genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameUpperCase}", strings.ToUpper(strings.Replace(string(sourceFilePrefDesc.Package()), ".", "", -1)), -1)))
								genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameDoubleSlashCamelCase}", ToCamelCase(string(sourceFilePrefDesc.Package()), ".", "\\"), -1)))
								genFileProto.Options.ProtoReflect().Set(v.desc, pref.ValueOf(strings.Replace(genFileProto.Options.ProtoReflect().Get(v.desc).String(), "{PackageNameDoubleColonCamelCase}", ToCamelCase(string(sourceFilePrefDesc.Package()), ".", "::"), -1)))
								fmt.Println("sourceFilePrefDesc.Options().ProtoReflect()", k, v.val.String(), genFileProto.Options.ProtoReflect().Get(v.desc), ToCamelCase(string(sourceFilePrefDesc.Package()), ".", ""))
							}
						}
						// Обработаем опцию  entity.feature.api_specification
						genEntityApiSpec(val, msgDesc, sourceFileJhumpDesc, genFileProto, googleApiAnnotationsFiles[0], genFileComments, filesJhumpDesc, addProtoRoot)
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
					}
					// если есть entity.feature.aml_specification
					if val.fullName == "entity.feature.aml_specification" {
						fmt.Println("entity.feature.aml_specification not implemented")
					}
				}
			}

		}

		if genFileProto, ok := genFilesDescProto[sourceFileName]; ok {
			// Добавим типы, которые пришли с шаблонами, но не внутри шаблона
			for _, v := range filesAddProtoRoot[sourceFileName] {
				genFileProto.MessageType = append(genFileProto.MessageType, protodesc.ToDescriptorProto(v))
			}

			// уберем опции entity.feature с entity, они не нужны в генерации
			for i := range genFileProto.MessageType {
				m := getFieldMap(genFileProto.MessageType[i].Options.ProtoReflect())
				for _, val := range m {
					if strings.Contains(string(val.fullName), "entity.feature.") {
						genFileProto.MessageType[i].Options.ProtoReflect().Clear(val.desc)
					}
				}
			}
			// убираем импорт entity-tmpl/entity-feature.proto
			for i := range genFileProto.Dependency {
				if genFileProto.Dependency[i] == "entity-tmpl/entity-feature.proto" {
					genFileProto.Dependency[i] = genFileProto.Dependency[len(genFileProto.Dependency)-1]
				}
			}
			deps := genFileProto.Dependency[:len(genFileProto.Dependency)-1]
			genFileProto.Dependency = deps

			var dpj []*desc.FileDescriptor
			for k, v := range filesJhumpDesc {
				dpj = append(dpj, v)
				fmt.Println("genFileProto.Dependency", k)
			}

			genFileJhumpDesc, err := desc.CreateFileDescriptor(genFileProto, dpj...)
			if err != nil {
				panic(err)
			}

			genFileBuilder, err := builder.FromFile(genFileJhumpDesc)
			if err != nil {
				panic(err)
			}
			for k, v := range genFileComments {
				fmt.Println("genFileComments", sourceFileName, k, v)
			}

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

			genFileDesc, err := genFileBuilder.Build()
			if err != nil {
				panic(err)
			}

			p := new(protoprint.Printer)
			p.SortElements = true
			p.Indent = "    "
			protoStr, err := p.PrintProtoToString(genFileDesc)
			if err != nil {
				panic(err)
			}
			// TODO: Добавить параметры генерации в коммент файла
			protoStr = "// " + genFileComments[genFileJhumpDesc.GetName()] + "\n\n" + protoStr
			fmt.Println("print Proto", protoStr)
			m[sourceFileName] = protoStr

		}

	}
	fmt.Println("Generation end")
	return m
}

func main() {
	BuildEntityFeatures("./templates", []string{".", "proto_deps"})
}
