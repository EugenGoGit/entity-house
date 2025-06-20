package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	//     "google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile"
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
	// "google.golang.org/protobuf/types/dynamicpb"
	// "github.com/bufbuild/protocompile/protoutil"
	"google.golang.org/protobuf/encoding/protojson"
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
//             нельзя собрать
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

func printFieldMap(n string, m map[string]Field) {
	for key, val := range m {
		if val.desc.Kind() == pref.MessageKind {
			fmt.Println(n, key, "  ", val.desc.FullName(), "  ", val.val.Message().Descriptor().FullName(), "  ", val.val.String())
		} else {
			fmt.Println(n, key, "  ", val.desc.FullName(), "  ", val.val.String())
		}
		fmt.Println()
	}
}

func printEnumEl(el Field) {
	fmt.Println(el.desc.FullName(), " ", el.desc.Enum().FullName(), " ", el.val, " ", el.desc.Enum().Values().Get(int(el.val.Enum())).Name())
	fmt.Println()
}

func createMessageDescByTemplate(template *pref.Value, messageName string, replaceFromType string, replaceTo *descriptorpb.DescriptorProto) *descriptorpb.DescriptorProto {
	var requestMessageProtodesc *descriptorpb.DescriptorProto
	fmt.Println(template)
	if template.String() != "<nil>" {
		fmt.Println(messageName)
		// Берем шаблон из поля, поле там одно
		for _, v := range getFieldMap(template.Message()) {
			requestMessageProtodesc = protodesc.ToDescriptorProto(v.val.Message().Descriptor())
			break
		}
		requestMessageProtodesc.Name = &messageName
		ReplaceFieldDescriptor(requestMessageProtodesc, replaceFromType, replaceTo)
		fmt.Println("requestMessageProtodesc", requestMessageProtodesc)
	} else {
		// Если template не задан используем пустой message

		requestMessageProtodesc = &descriptorpb.DescriptorProto{
			Name: &messageName,
		}
		fmt.Println("requestTemplateVal empty")
	}
	return requestMessageProtodesc
}

func OptionExclude(messageDesc pref.MessageDescriptor, excludeOptFullName string) *descriptorpb.MessageOptions {
	var opt descriptorpb.MessageOptions = descriptorpb.MessageOptions{}
	entityOpts := getFieldMap(messageDesc.Options().ProtoReflect())
	for _, optField := range entityOpts {
		if optField.fullName != "entity.feature.api_service" {
			var valOptS string
			if optField.desc.Kind() == pref.MessageKind {
				valOpt, _ := protojson.MarshalOptions{
					Multiline:     true,
					UseProtoNames: true,
				}.Marshal(optField.val.Message().Interface())
				valOptS = string(valOpt)
			} else {
				valOptS = optField.val.String()
			}
			n := string(optField.desc.Name())
			e := strings.Index(string(optField.desc.FullName()), "google.protobuf.MessageOptions") == -1
			if e {
				n = string(optField.desc.FullName())
			}
			opt.UninterpretedOption = append(opt.UninterpretedOption,
				&descriptorpb.UninterpretedOption{
					Name: []*descriptorpb.UninterpretedOption_NamePart{
						{
							NamePart:    &n,
							IsExtension: &e,
						},
					},
					IdentifierValue: &valOptS,
				},
			)

		}
	}
	return &opt
}

func ReplaceFieldDescriptor(source *descriptorpb.DescriptorProto, replaceFromType string, replaceTo *descriptorpb.DescriptorProto) {
	for i := range source.Field {
		fn := source.Field[i].TypeName
		if string(*fn) == replaceFromType {
			n := replaceTo.GetName()
			source.Field[i].TypeName = &n
		}
	}
}

func getUniqueFieldGroup(entityDesc pref.MessageDescriptor, uniqueFieldGroupEl pref.EnumNumber) []string {
	var res []string
	var m map[pref.EnumNumber][]string = make(map[pref.EnumNumber][]string)
	var min pref.EnumNumber = 10000
	for i := range entityDesc.Fields().Len() {
		optMap := getFieldMap(entityDesc.Fields().Get(i).Options().ProtoReflect())
		if val, ok := optMap["unique_field_group"]; ok {
			m[val.val.Enum()] = append(m[val.val.Enum()], string(entityDesc.Fields().Get(i).Name()))
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

func getKeyFields(keyFieldsDefinition *pref.Value, entityDesc pref.MessageDescriptor) []string {
	var res []string
	if keyFieldsDefinition.String() == "<nil>" {
		res = getUniqueFieldGroup(entityDesc, 0)
	} else {
		fieldMap := getFieldMap(keyFieldsDefinition.Message())
		if val, ok := fieldMap["key_field_list"]; ok {
			keyFieldsFieldMap := getFieldMap(val.val.Message())
			if val1, ok := keyFieldsFieldMap["key_fields"]; ok {
				for i := range val1.val.List().Len() {
					res = append(res, val1.val.List().Get(i).String())
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

func main() {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	fmt.Println("filepath.Dir(ex)")
	fmt.Println(exPath)
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	fmt.Println("os.Getwd()")
	fmt.Println(path)

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
			ImportPaths: []string{
				".", "proto_deps", // "entity-tmpl",
			},
		}),
		Reporter:       rep,
		SourceInfoMode: protocompile.SourceInfoMode(2),
		RetainASTs:     true,
	}

	entityFiles, err := compiler.Compile(context.Background(), "templates/device/v1/deviceapis_device_dtmf_v1.proto")
	if err != nil {
		panic(err)
	}
	fmt.Println("entityFiles", entityFiles)

	for _, entityFileDesc := range entityFiles {
		entitiesToGenerateApiFeature := make(map[pref.MessageDescriptor]pref.Message)
		for i := range entityFileDesc.Messages().Len() {
			msgDesc := entityFileDesc.Messages().Get(i)
			// смотрим какие именно заявлены фичи сущности
			// ищем опции entity.feature
			msgOptsMap := getFieldMap(msgDesc.Options().ProtoReflect())
			// если есть entity.feature.api_service, считаем, что в поле api_service описание сервиса
			// считаем, что опция Kind message
			// считаем, что данный Message описывает сущность для которой нужен сервис
			if val, ok := msgOptsMap["api_service"]; ok && val.fullName == "entity.feature.api_service" {
				// берем поле фичи, в котором описан сервис
				// считаем, что поле там только одно
				var serviceOptVal pref.Message
				for _, value := range getFieldMap(msgOptsMap["api_service"].val.Message()) {
					serviceOptVal = value.val.Message()
					break
				}
				entitiesToGenerateApiFeature[msgDesc] = serviceOptVal
			}
		}

		if len(entitiesToGenerateApiFeature) > 0 {
			// Если есть сущности с фичами entity.feature, то создаем файл для генерации
			sourcefileDescriptorProto := protodesc.ToFileDescriptorProto(entityFileDesc)
			fmt.Println("sourcefileDescriptorProto", sourcefileDescriptorProto.MessageType[0].Options.ProtoReflect())

			sourcefileDescriptorProto.Dependency = nil

			sourcefileDescFd, err := desc.CreateFileDescriptor(sourcefileDescriptorProto)
			if err != nil {
				panic(err)
			}

			genFileProto := &descriptorpb.FileDescriptorProto{
				Syntax:  proto.String("proto3"),
				Name:    proto.String(string(entityFileDesc.Package()) + "_gen.proto"),
				Package: proto.String(string(entityFileDesc.Package())),
			}

			var genFileComments map[string]string = make(map[string]string)

			// Создаем сервисы для каждой сущности
			for entityDesc, serviceDef := range entitiesToGenerateApiFeature {

				fmt.Println(entityDesc.FullName(), serviceDef.Descriptor().FullName())
				// Создаем сервис
				// в опциях шаблон имени
				serviceOptsMap := getFieldMap(serviceDef.Descriptor().Options().ProtoReflect())
				serviceName := strings.Replace(serviceOptsMap["name_template"].val.String(), "{EntityName}", string(entityDesc.Name()), -1)

				// в полях сервиса требуемые методы в method_set
				serviceFieldsMap := getFieldMap(serviceDef)
				methodsDefMap := getFieldMap(serviceFieldsMap["method_set"].val.Message())
				// Корневой httpPath в http_root, он же - признак добавления опций http
				httpRoot := serviceFieldsMap["http_root"].val.String()
				// Определение уникальных полей сервиса, будет использовано, если не переопределено на уровне метода
				serviceKeyFieldsDefinition := serviceFieldsMap["key_fields_definition"].val

				genServiceProto := &descriptorpb.ServiceDescriptorProto{
					Name: proto.String(serviceName),
				}
				genFileComments[serviceName] = "Сервис управления сущностью " + string(entityDesc.Name())

				// Найдем комменты Entity
				var num int
				for i := range sourcefileDescFd.GetMessageTypes() {
					if sourcefileDescFd.GetMessageTypes()[i].GetName() == string(entityDesc.Name()) {
						num = i
					}
				}
				entityMessageDesc := sourcefileDescFd.GetMessageTypes()[num]
				genFileComments[string(entityDesc.Name())] = *entityMessageDesc.GetSourceInfo().LeadingComments

				// Добавим сам Entity
				// Сохраним все опции, кроме entity.feature.api_service, которую используем в данной генерации
				entityMessageProtodesc := protodesc.ToDescriptorProto(entityDesc)
				entityMessageProtodesc.Options = OptionExclude(entityDesc, "entity.feature.api_service")

				for _, methodDef := range methodsDefMap {
					// в полях заданы параметры метода
					methodFieldMap := getFieldMap(methodDef.val.Message())
					// переопределяем Определение уникальных полей сервиса
					keyFieldsDefinition := serviceKeyFieldsDefinition
					if val, ok := methodFieldMap["key_fields_definition"]; ok {
						keyFieldsDefinition = val.val
					}
					fmt.Println("method_unique_key", keyFieldsDefinition)
					// в опциях параметры генерации метода
					methodOptsMap := getFieldMap(methodDef.val.Message().Descriptor().Options().ProtoReflect())
					// в полях entity.api.method_component_template_set шаблоны компонент метода
					methodTemplatesMap := getFieldMap(methodOptsMap["method_component_template_set"].val.Message())
					// шаблон имени в name_template
					methodName := strings.Replace(methodTemplatesMap["name_template"].val.String(), "{EntityName}", string(entityDesc.Name()), -1)
					// methodFullName := strings.Replace(methodTemplatesMap["name_template"].val.String(), "{EntityName}", string(entityDesc.FullName()), -1)
					// шаблон имени запроса в request_dto_name_template
					requestName := strings.Replace(methodTemplatesMap["request_dto_name_template"].val.String(), "{EntityName}", string(entityDesc.Name()), -1)
					// requestFullName := strings.Replace(methodTemplatesMap["request_dto_name_template"].val.String(), "{EntityName}", string(entityDesc.FullName()), -1)
					// шаблон имени ответа в response_dto_name_template
					responseName := strings.Replace(methodTemplatesMap["response_dto_name_template"].val.String(), "{EntityName}", string(entityDesc.Name()), -1)
					// responseFullName := strings.Replace(methodTemplatesMap["response_dto_name_template"].val.String(), "{EntityName}", string(entityDesc.FullName()), -1)
					// шаблон запроса в request_template
					// там только одно поле
					tmpl := methodTemplatesMap["request_template"]
					requestMessageProtodesc := createMessageDescByTemplate(
						&tmpl.val,
						requestName,
						".entity.feature.api.options.EntityKeyDescriptor",
						entityMessageProtodesc,
					)
					genFileComments[requestName] = "Запрос метода " + methodName

					tmpl = methodTemplatesMap["response_template"]
					responseMessageProtodesc := createMessageDescByTemplate(
						&tmpl.val,
						responseName,
						".entity.feature.api.options.EntityKeyDescriptor",
						entityMessageProtodesc,
					)
					genFileComments[responseName] = "Ответ на запрос метода " + methodName

					genFileProto.MessageType = append(genFileProto.MessageType, requestMessageProtodesc)
					genFileProto.MessageType = append(genFileProto.MessageType, responseMessageProtodesc)

					genMethodProto := &descriptorpb.MethodDescriptorProto{
						Name:            proto.String(methodName),
						InputType:       proto.String(requestName),
						OutputType:      proto.String(responseName),
						ServerStreaming: proto.Bool(true),
						ClientStreaming: proto.Bool(true),
					}

					// в полях entity.api.http_rule http опции google.api.HttpRule
					if val, ok := methodOptsMap["http_rule"]; ok {
						if httpRoot != "" && httpRoot != "<nil>" {
							keyFieldList := getKeyFields(&keyFieldsDefinition, entityDesc)
							var keyFieldPath string
							for i := range keyFieldList {
								keyFieldPath = keyFieldPath + keyFieldList[i] + "/{" + keyFieldList[i] + "}"
								if i != len(keyFieldList)-1 {
									keyFieldPath = keyFieldPath + "/"
								}
							}
							methodHttpRuleMap := getFieldMap(val.val.Message())
							n := "google.api.http"
							e := true
							var valOpt string = "{ "
							for k, v := range methodHttpRuleMap {
								valOpt = valOpt + k + ": \"" +
									strings.Replace(strings.Replace(v.val.String(), "{HttpRoot}", httpRoot, -1), "{KeyFields}", keyFieldPath, -1) +
									"\", "
							}
							valOpt = valOpt + "}"
							var opt descriptorpb.MethodOptions = descriptorpb.MethodOptions{
								UninterpretedOption: []*descriptorpb.UninterpretedOption{
									{Name: []*descriptorpb.UninterpretedOption_NamePart{
										{
											NamePart:    &n,
											IsExtension: &e,
										},
									},
										IdentifierValue: &valOpt,
									},
								},
							}
							genMethodProto.Options = &opt
						}
					}
					// в поле entity.rules.key_field_behavior поведение полей ключа, требуемое для данного метода
					printEnumEl(methodOptsMap["key_field_behavior"])
					// genMethodProto.Options
					genServiceProto.Method = append(genServiceProto.Method, genMethodProto)
					genFileComments[methodName] = "Метод " + methodName
				}

				genFileProto.Service = append(genFileProto.Service, genServiceProto)

				genFileProto.MessageType = append(genFileProto.MessageType, entityMessageProtodesc)
			}

			genFileDescFd, err := desc.CreateFileDescriptor(genFileProto)
			if err != nil {
				panic(err)
			}

			genFileBuilder, err := builder.FromFile(genFileDescFd)
			if err != nil {
				panic(err)
			}

			for i := range genFileDescFd.GetServices() {
				if val, ok := genFileComments[genFileDescFd.GetServices()[i].GetName()]; ok {
					genFileBuilder.GetService(genFileDescFd.GetServices()[i].GetName()).SetComments(builder.Comments{LeadingComment: val})
				}
				for j := range genFileDescFd.GetServices()[i].GetMethods() {
					if val, ok := genFileComments[genFileDescFd.GetServices()[i].GetMethods()[j].GetName()]; ok {
						genFileBuilder.GetService(genFileDescFd.GetServices()[i].GetName()).GetMethod(genFileDescFd.GetServices()[i].GetMethods()[j].GetName()).SetComments(builder.Comments{LeadingComment: val})
					}
				}
			}

			for i := range genFileDescFd.GetMessageTypes() {
				if val, ok := genFileComments[genFileDescFd.GetMessageTypes()[i].GetName()]; ok {
					genFileBuilder.GetMessage(genFileDescFd.GetMessageTypes()[i].GetName()).SetComments(builder.Comments{LeadingComment: val})
				}
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
			fmt.Println("print Proto", protoStr)
			// protoStr2, err := p.PrintProtoToString(sourcefileDescFd)
			// if err != nil {
			// 	panic(err)
			// }

			// fmt.Println("protoStr2", protoStr2)
			// fmt.Println("genFileDesc", genFileDesc)

		}

	}
	// m := temlpateDesc.Messages().ByName("MethodCollection")
	// fmt.Println(m)
	fmt.Println("END")
}
