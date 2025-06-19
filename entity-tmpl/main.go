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

// func getMessageBuilderFromTemplate(templateMessage Field, messageTemplateDescFd *desc.FileDescriptor, name string) *builder.MessageBuilder {
// 	var template pref.Message
// 	for _, value := range getFieldMap(templateMessage.val.Message()) {
// 		template = value.val.Message()
// 		break
// 	}

// 	var num int
// 	for i := range messageTemplateDescFd.GetMessageTypes() {
// 		if messageTemplateDescFd.GetMessageTypes()[i].GetName() == string(template.Descriptor().Name()) {
// 			num = i
// 			break
// 		}
// 	}
// 	messageBuilder, err := builder.FromMessage(messageTemplateDescFd.GetMessageTypes()[num])
// 	if err != nil {
// 		panic(err)
// 	}
// 	messageBuilder.SetName(name)
// 	desc.ToFileDescriptorSet()
// 	return messageBuilder
// }

func printEnumEl(el Field) {
	fmt.Println(el.desc.FullName(), " ", el.desc.Enum().FullName(), " ", el.val, " ", el.desc.Enum().Values().Get(int(el.val.Enum())).Name())
	fmt.Println()
}

// type ServiceDescriptorProtoWComment struct {
// 	proto   *descriptorpb.ServiceDescriptorProto
// 	comment string
// }

func ReplaceFieldDescriptor(source *descriptorpb.DescriptorProto, replaceFromType string, replaceTo *descriptorpb.DescriptorProto) {
	for i := range source.Field {
		fn:=source.Field[i].TypeName
		if string(*fn) == replaceFromType {
			n := replaceTo.GetName()
			source.Field[i].TypeName = &n
		}
	}
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

	// reph := reporter.NewHandler(rep)

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

	// compiler2 := protocompile.Compiler{
	// 	Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
	// 		ImportPaths: []string{
	// 			".", //"proto_deps",
	// 		},
	// 	}),
	// 	Reporter:       rep,
	// 	SourceInfoMode: protocompile.SourceInfoMode(2),
	// 	RetainASTs:     true,
	// }

	entityFiles, err := compiler.Compile(context.Background(), "templates/device/v1/deviceapis_device_dtmf_v1.proto")
	if err != nil {
		panic(err)
	}
	fmt.Println("entityFiles", entityFiles)
	messageTemplateFiles, err := compiler.Compile(context.Background(), "entity-tmpl/entity-feature-api-template.proto")
	if err != nil {
		panic(err)
	}
	fmt.Println("messageTemplateFiles", messageTemplateFiles)
	field_behaviorFiles, err := compiler.Compile(context.Background(), "google/api/field_behavior.proto")
	if err != nil {
		panic(err)
	}
	fmt.Println("field_behaviorFiles", field_behaviorFiles)
	// descriptorFiles, err := compiler2.Compile(context.Background(), "google/protobuf/descriptor.proto")
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// fmt.Println("descriptorFiles", descriptorFiles)
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
			messageTemplateDescriptorProto := protodesc.ToFileDescriptorProto(messageTemplateFiles[0])

			// fieldBehaviorFilesDescriptorProto := protodesc.ToFileDescriptorProto(field_behaviorFiles[0])
			// descriptorDescriptorProto := protodesc.ToFileDescriptorProto(descriptorFiles[0])
			s := "fgfgfhfhfhfhfhfhfh33333"
			sourcefileDescriptorProto.SourceCodeInfo.Location[12].LeadingComments = &s

			// m:=internal.CreateSourceInfoMap(sourcefileDescriptorProto)
			//  = append(sourcefileDescriptorProto.SourceCodeInfo.Location,
			// 	&descriptorpb.SourceCodeInfo_Location{
			// 		Path:            []int32{4, 3, 2, 0, 6},
			// 		Span:            []int32{20, 2, 21},
			// 		LeadingComments: &s,
			// 	},
			// )
			// fileDescriptorProto := &descriptorpb.FileDescriptorProto{
			// 	// оставляем от исходного
			// 	Name:           sourcefileDescriptorProto.Name,
			// 	Package:        sourcefileDescriptorProto.Package,
			// 	// Dependency:     sourcefileDescriptorProto.Dependency,
			// 	Syntax:         sourcefileDescriptorProto.Syntax,
			// 	MessageType:    sourcefileDescriptorProto.MessageType,
			// 	Service:        sourcefileDescriptorProto.Service,
			// 	Extension:      sourcefileDescriptorProto.Extension,
			// 	Options:        sourcefileDescriptorProto.Options,
			// 	SourceCodeInfo: sourcefileDescriptorProto.SourceCodeInfo,
			// }

			sourcefileDescriptorProto.Dependency = nil
			// messageTemplateDescriptorProto.Dependency = nil
			// descriptorDescFd, err := desc.CreateFileDescriptor(descriptorDescriptorProto)
			// if err != nil {
			// 	panic(err)
			// }
			// fieldBehaviorDescFd, err := desc.CreateFileDescriptor(fieldBehaviorFilesDescriptorProto)
			// if err != nil {
			// 	panic(err)
			// }

			fmt.Println("messageTemplateDescriptorProto", messageTemplateDescriptorProto)
			for i := range messageTemplateDescriptorProto.MessageType {
				for j := range messageTemplateDescriptorProto.MessageType[i].Field {
					fmt.Println("messageTemplateDescriptorProto.MessageType[i].Field[j].TypeName",
						messageTemplateDescriptorProto.MessageType[i].Field[j].Type.Descriptor().Name())
				}
			}

			sourcefileDescFd, err := desc.CreateFileDescriptor(sourcefileDescriptorProto)
			if err != nil {
				panic(err)
			}

			messageTemplateDescFd, err := desc.CreateFileDescriptor(messageTemplateDescriptorProto)
			if err != nil {
				panic(err)
			}
			for i := range messageTemplateDescFd.GetMessageTypes() {
				fmt.Println("messageTemplateDescFd.GetMessageTypes()", messageTemplateDescFd.GetMessageTypes()[i])
			}

			// sourcefileDescriptor, _ := desc.CreateFileDescriptor(sourcefileDescriptorProto)
			// genFileBuilder := builder.NewFile(string(entityFileDesc.Package()) + "_gen.proto")
			// genFileBuilder.SetPackageName(string(entityFileDesc.Package()))
			// genFileBuilder.SetProto3(true)
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
				// serviceFullName := strings.Replace(serviceOptsMap["name_template"].val.String(), "{EntityName}", string(entityDesc.FullName()), -1)

				// pfs := builder.NewService(serviceName)
				// pfs.SetComments(builder.Comments{
				// 	LeadingComment: "Сервис управления сущностью " + string(entityDesc.Name()),
				// })
				// genFileBuilder.AddService(pfs)
				// в полях сервиса требуемые методы в method_set
				serviceFieldsMap := getFieldMap(serviceDef)
				methodsDefMap := getFieldMap(serviceFieldsMap["method_set"].val.Message())
				// Корневой httpPath в http_root, он же - признак добавления опций http
				httpRoot := serviceFieldsMap["http_root"].val.String()
				// Уникальный ключ используется, если не переопределен на уровне метода
				serviceUniqueKey := serviceFieldsMap["key_fields"].val.List()

				genServiceProto := &descriptorpb.ServiceDescriptorProto{
					Name: proto.String(serviceName),
				}

				// Найдем комменты Entity
				var num int
				for i := range sourcefileDescFd.GetMessageTypes() {
					if sourcefileDescFd.GetMessageTypes()[i].GetName() == string(entityDesc.Name()) {
						num = i
					}
				}
				entityMessageDesc := sourcefileDescFd.GetMessageTypes()[num]
				genFileComments[string(entityDesc.Name())] = *entityMessageDesc.GetSourceInfo().LeadingComments
				// messageBuilder, err := builder.FromMessage(entityMessageDesc)
				// if err != nil {
				// 	panic(err)
				// }
				// genFileBuilder.AddMessage(messageBuilder)

				// Добавим сам Entity

				// Сохраним все опции, кроме entity.feature.api_service, которую используем в данной генерации
				var opt descriptorpb.MessageOptions = descriptorpb.MessageOptions{}
				entityOpts := getFieldMap(entityDesc.Options().ProtoReflect())
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
				entityMessageProtodesc := protodesc.ToDescriptorProto(entityDesc)
				entityMessageProtodesc.Options = &opt

				genFileComments[serviceName] = "Сервис управления сущностью " + string(entityDesc.Name())

				for _, methodDef := range methodsDefMap {
					method_unique_key := serviceUniqueKey
					// в полях заданы параметры метода
					methodFieldMap := getFieldMap(methodDef.val.Message())
					// переопределяем уникальный ключ
					if val, ok := methodFieldMap["unique_key"]; ok {
						method_unique_key = val.val.List()
					}
					fmt.Println("method_unique_key", method_unique_key)
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
					// var requestMessageBuilder *builder.MessageBuilder
					var requestMessageProtodesc *descriptorpb.DescriptorProto
					if val, ok := methodTemplatesMap["request_template"]; ok {
						// Берем шаблон из поля, поле там одно
						for _, v := range getFieldMap(methodTemplatesMap["request_template"].val.Message()) {
							requestMessageProtodesc = protodesc.ToDescriptorProto(v.val.Message().Descriptor())
							break
						}
						requestMessageProtodesc.Name = &requestName
						ReplaceFieldDescriptor(requestMessageProtodesc, ".entity.feature.api.options.EntityKeyDescriptor", entityMessageProtodesc)
						fmt.Println("requestMessageProtodesc", val.val.Message().Descriptor())
						// requestMessageProtodesc.NestedType
						fmt.Println("requestMessageProtodesc", requestMessageProtodesc)
						// dd := desc.MessageDescriptor{proto:requestMessageProtodesc}
						// requestMessageBuilder = getMessageBuilderFromTemplate(val, messageTemplateDescFd, requestName)
					} else {
						// Если template не задан используем пустой message
						// requestMessageBuilder = builder.NewMessage(requestName)
						requestMessageProtodesc = &descriptorpb.DescriptorProto{
							Name: &requestName,
						}
						fmt.Println("requestTemplateVal empty")
					}
					genFileComments[requestName] = "Запрос метода " + methodName

					// fmt.Println("requestMessageBuilder", requestMessageBuilder)
					// var responseMessageBuilder *builder.MessageBuilder
					var responseMessageProtodesc *descriptorpb.DescriptorProto
					if val, ok := methodTemplatesMap["response_template"]; ok {
						responseMessageProtodesc = protodesc.ToDescriptorProto(val.val.Message().Descriptor())
						fmt.Println("responseMessageProtodesc", val.val.Message().Descriptor())
						// responseMessageProtodesc.NestedType
						fmt.Println("responseMessageProtodesc", responseMessageProtodesc.Field[0])
						// dd := desc.MessageDescriptor{proto:responseMessageProtodesc}
						// responseMessageBuilder = getMessageBuilderFromTemplate(val, messageTemplateDescFd, responseName)
					} else {
						// Если template не задан используем пустой message
						// responseMessageBuilder = builder.NewMessage(responseName)
						responseMessageProtodesc = &descriptorpb.DescriptorProto{
							Name: &responseName,
						}
						fmt.Println("responseTemplateVal empty")
					}
					genFileComments[responseName] = "Запрос метода " + methodName

					// genFileBuilder.AddMessage(requestMessageBuilder)
					// genFileBuilder.AddMessage(responseMessageBuilder)
					genFileProto.MessageType = append(genFileProto.MessageType, requestMessageProtodesc)
					genFileProto.MessageType = append(genFileProto.MessageType, responseMessageProtodesc)

					// pfm := builder.NewMethod(methodName,
					// 	builder.RpcTypeMessage(requestMessageBuilder, false),
					// 	builder.RpcTypeMessage(responseMessageBuilder, false),
					// )
					// pfm.SetComments(builder.Comments{
					// 	LeadingComment: "Метод " + methodName,
					// })

					genMethodProto := &descriptorpb.MethodDescriptorProto{
						Name:            proto.String(methodName),
						InputType:       proto.String(requestName),
						OutputType:      proto.String(responseName),
						ServerStreaming: proto.Bool(true),
						ClientStreaming: proto.Bool(true),
					}

					// pfs.AddMethod(pfm)
					// в полях entity.api.http_rule http опции google.api.HttpRule
					if val, ok := methodOptsMap["http_rule"]; ok {
						if httpRoot != "" && httpRoot != "<nil>" {
							var keyFields string
							for i := 0; i < method_unique_key.Len(); i++ {
								keyFields = keyFields + method_unique_key.Get(i).String() + "/{" + method_unique_key.Get(i).String() + "}"
								if i != method_unique_key.Len()-1 {
									keyFields = keyFields + "/"
								}
							}
							methodHttpRuleMap := getFieldMap(val.val.Message())
							n := "google.api.http"
							e := true
							var valOpt string = "{ "
							for k, v := range methodHttpRuleMap {
								valOpt = valOpt + k + ": \"" +
									strings.Replace(strings.Replace(v.val.String(), "{HttpRoot}", httpRoot, -1), "{KeyFields}", keyFields, -1) +
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
