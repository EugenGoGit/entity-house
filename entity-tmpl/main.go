package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
	// "github.com/jhump/protoreflect/desc/builder"
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
	replaceKeysToFields []*descriptorpb.FieldDescriptorProto,
) *descriptorpb.DescriptorProto {
	var messageProtodesc *descriptorpb.DescriptorProto
	var templateTypeName string
	if templateParent.String() != "<nil>" {
		// Берем шаблон из поля, поле там одно
		for _, v := range getFieldMap(templateParent.Message()) {
			messageProtodesc = protodesc.ToDescriptorProto(v.val.Message().Descriptor())
			templateTypeName = string(v.val.Message().Descriptor().FullName())
			break
		}
		messageProtodesc.Name = &messageName
		changeFieldTypeNameNested(messageProtodesc,
			".entity.feature.api.options.EntityDescriptor",
			".entity.feature.api.options.EntityKeyDescriptor",
			*replaceTo.Name,
			replaceKeysToFields,
			templateTypeName,
			messageName,
			messageFullName)
	} else {
		// Если template не задан используем пустой message
		messageProtodesc = &descriptorpb.DescriptorProto{
			Name: &messageName,
		}
	}
	return messageProtodesc
}

func changeFieldTypeNameNested(messageDescProto *descriptorpb.DescriptorProto,
	replaceFromType string,
	replaceKeysFromType string,
	replaceToType string,
	replaceKeysToFields []*descriptorpb.FieldDescriptorProto,
	topLevelNameFromType string,
	topLevelNameToType string,
	topLevelFullNameToType string) {
	for i := range messageDescProto.Field {
		if *messageDescProto.Field[i].Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			if *messageDescProto.Field[i].TypeName == replaceFromType {
				messageDescProto.Field[i].TypeName = &replaceToType
			} else {
				s := strings.Replace(*messageDescProto.Field[i].TypeName, "."+topLevelNameFromType+".", topLevelNameToType+".", -1)
				messageDescProto.Field[i].TypeName = &s
			}
			if *messageDescProto.Field[i].TypeName == replaceKeysFromType {
				n := messageDescProto.Field[i].Number
				messageDescProto.Field[i] = replaceKeysToFields[0]
				messageDescProto.Field[i].Number = n
				for j := 1; j < len(replaceKeysToFields); j++ {
					if len(messageDescProto.Field) == i+1 {
						messageDescProto.Field = append(messageDescProto.Field, replaceKeysToFields[j])
					} else {
						messageDescProto.Field = append(messageDescProto.Field[:i+1], messageDescProto.Field[i:]...) // index < len(a)
						messageDescProto.Field[i+1] = replaceKeysToFields[j]
					}
				}
				for j := range messageDescProto.Field {
					n := int32(j + 1)
					if n > *messageDescProto.Field[j].Number {
						messageDescProto.Field[j].Number = &n
					}
				}
			}
		}

	}
	for i := range messageDescProto.NestedType {
		s := strings.Replace(*messageDescProto.NestedType[i].Name, topLevelNameFromType, topLevelFullNameToType, -1)
		messageDescProto.NestedType[i].Name = &s
		changeFieldTypeNameNested(messageDescProto.NestedType[i],
			replaceFromType,
			replaceKeysFromType,
			replaceToType,
			replaceKeysToFields,
			topLevelNameFromType,
			topLevelNameToType,
			topLevelFullNameToType)
	}
}

func getUniqueFieldGroup(entityDesc pref.MessageDescriptor, uniqueFieldGroupEl pref.EnumNumber) []*descriptorpb.FieldDescriptorProto {
	var res []*descriptorpb.FieldDescriptorProto
	var m map[pref.EnumNumber][]*descriptorpb.FieldDescriptorProto = make(map[pref.EnumNumber][]*descriptorpb.FieldDescriptorProto)
	var min pref.EnumNumber = 10000
	for i := range entityDesc.Fields().Len() {
		optMap := getFieldMap(entityDesc.Fields().Get(i).Options().ProtoReflect())
		if val, ok := optMap["unique_field_group"]; ok {
			m[val.val.Enum()] = append(m[val.val.Enum()], protodesc.ToFieldDescriptorProto(entityDesc.Fields().Get(i)))
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

func getKeyFields(keyFieldsDefinition *pref.Value, entityDesc pref.MessageDescriptor) []*descriptorpb.FieldDescriptorProto {
	var res []*descriptorpb.FieldDescriptorProto
	if keyFieldsDefinition.String() == "<nil>" {
		res = getUniqueFieldGroup(entityDesc, 0)
	} else {
		fieldMap := getFieldMap(keyFieldsDefinition.Message())
		if val, ok := fieldMap["key_field_list"]; ok {
			keyFieldsFieldMap := getFieldMap(val.val.Message())
			if val1, ok := keyFieldsFieldMap["key_fields"]; ok {
				for i := range val1.val.List().Len() {
					res = append(res, protodesc.ToFieldDescriptorProto(entityDesc.Fields().ByName(pref.Name(val1.val.List().Get(i).String()))))
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

func main() {

	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	fmt.Println("os.Getwd()", path)

	entityFileDirName := "./templates"
	var entityProtoFileNames []string
	err1 := filepath.Walk(entityFileDirName,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			split := strings.Split(path, ".")
			if !info.IsDir() && split[len(split)-1] == "proto" {
				entityProtoFileNames = append(entityProtoFileNames, strings.Replace(path, "\\", "/", -1))
			}
			return nil
		})
	if err1 != nil {
		log.Println(err1)
	}

	fmt.Println("entityProtoFileNames", entityProtoFileNames)

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
				".", "proto_deps",
			},
		}),
		Reporter:       rep,
		SourceInfoMode: protocompile.SourceInfoMode(1),
		RetainASTs:     true,
	}

	var entityFilesPRF map[string]linker.File = make(map[string]linker.File)
	var entityFilesJ map[string]*desc.FileDescriptor = make(map[string]*desc.FileDescriptor)
	for i := range entityProtoFileNames {
		entityFiles, err := compiler.Compile(context.Background(), entityProtoFileNames[i])
		if err != nil {
			panic(err)
		}
		entityFilesPRF[entityProtoFileNames[i]] = entityFiles[0]
		fd := protodesc.ToFileDescriptorProto(entityFiles[0])
		entityFilesJ[entityProtoFileNames[i]] = getProtoJ(compiler, fd, entityFilesJ)
	}

	googleApiAnnotationsFiles, err := compiler.Compile(context.Background(), "google/api/annotations.proto")
	if err != nil {
		panic(err)
	}

	for fileName, entityFileDesc := range entityFilesPRF {
		fmt.Println("entityFileDesc", entityFileDesc.FullName())
		// создаем файл для генерации
		sourcefileDescriptorProto := protodesc.ToFileDescriptorProto(entityFileDesc)
		fmt.Println("sourcefileDescriptorProto", sourcefileDescriptorProto.Dependency)

		sourcefileDescFd := entityFilesJ[fileName]
		fmt.Println("sourcefileDescFd", fileName)

		// создаем файл для генерации
		genFileProto := &descriptorpb.FileDescriptorProto{
			Syntax:     proto.String("proto3"),
			Name:       proto.String(string(entityFileDesc.Package()) + "_gen.proto"),
			Package:    proto.String(string(entityFileDesc.Package())),
			Dependency: sourcefileDescriptorProto.Dependency,
		}
		fmt.Println("genFileProto.Dependency1", genFileProto.Dependency)

		var genFileComments map[string]string = make(map[string]string)

		for i := range entityFileDesc.Messages().Len() {
			msgDesc := entityFileDesc.Messages().Get(i)
			// смотрим какие именно заявлены фичи сущности
			// ищем опции entity.feature
			msgOptsMap := getFieldMap(msgDesc.Options().ProtoReflect())
			// если есть entity.feature.api_service, считаем, что в поле api_service описание сервиса
			// считаем, что опция Kind = message
			// считаем, что данный Message описывает сущность для которой нужен сервис
			if val, ok := msgOptsMap["api_service"]; ok && val.fullName == "entity.feature.api_service" {
				// берем поле фичи, в котором описан сервис
				// считаем, что поле там только одно
				var serviceOptVal pref.Message
				for _, value := range getFieldMap(msgOptsMap["api_service"].val.Message()) {
					serviceOptVal = value.val.Message()
					break
				}
				// Создаем сервис
				// в опциях шаблон имени
				serviceOptsMap := getFieldMap(serviceOptVal.Descriptor().Options().ProtoReflect())
				serviceName := strings.Replace(serviceOptsMap["name_template"].val.String(), "{EntityName}", string(msgDesc.Name()), -1)

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
				genFileComments[serviceName] = "Сервис управления сущностью " + string(msgDesc.Name())

				// Найдем комменты Entity
				var num int
				for i := range sourcefileDescFd.GetMessageTypes() {
					if sourcefileDescFd.GetMessageTypes()[i].GetName() == string(msgDesc.Name()) {
						num = i
					}
				}
				entityMessageDesc := sourcefileDescFd.GetMessageTypes()[num]
				genFileComments[string(msgDesc.Name())] = *entityMessageDesc.GetSourceInfo().LeadingComments

				// Добавим сам Entity
				// Сохраним все опции, кроме entity.feature.api_service, которую используем в данной генерации
				// Уберем опцию, она обработана и не нужна в кодогенерации
				msgDesc.Options().ProtoReflect().Clear(msgOptsMap["api_service"].desc)
				entityMessageProtodesc := protodesc.ToDescriptorProto(msgDesc)

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
					methodName := strings.Replace(methodTemplatesMap["name_template"].val.String(), "{EntityName}", string(msgDesc.Name()), -1)
					// methodFullName := strings.Replace(methodTemplatesMap["name_template"].val.String(), "{EntityName}", string(msgDesc.FullName()), -1)
					// шаблон имени запроса в request_dto_name_template
					requestName := strings.Replace(methodTemplatesMap["request_dto_name_template"].val.String(), "{EntityName}", string(msgDesc.Name()), -1)
					requestFullName := strings.Replace(methodTemplatesMap["request_dto_name_template"].val.String(), "{EntityName}", string(msgDesc.FullName()), -1)
					// шаблон имени ответа в response_dto_name_template
					responseName := strings.Replace(methodTemplatesMap["response_dto_name_template"].val.String(), "{EntityName}", string(msgDesc.Name()), -1)
					// определяем список ключевых полей для метода
					keyFieldList := getKeyFields(&keyFieldsDefinition, msgDesc)
					// responseFullName := strings.Replace(methodTemplatesMap["response_dto_name_template"].val.String(), "{EntityName}", string(msgDesc.FullName()), -1)
					// шаблон запроса в request_template
					// там только одно поле
					tmpl := methodTemplatesMap["request_template"]
					requestMessageProtodesc := createMessageDescByTemplate(
						&tmpl.val,
						requestName,
						requestFullName,
						entityMessageProtodesc,
						keyFieldList,
					)
					genFileComments[requestName] = "Запрос метода " + methodName

					tmpl = methodTemplatesMap["response_template"]
					responseMessageProtodesc := createMessageDescByTemplate(
						&tmpl.val,
						responseName,
						requestFullName,
						entityMessageProtodesc,
						keyFieldList,
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
						Options:         &descriptorpb.MethodOptions{},
					}

					// в полях entity.api.http_rule http опции google.api.HttpRule
					// если заданы http_rule и httpRoot, то добавляем опции google.api.http для этого метода
					// с заменой по шаблону
					if val, ok := methodOptsMap["http_rule"]; ok {
						if httpRoot != "" && httpRoot != "<nil>" {
							var keyFieldPath string
							for i := range keyFieldList {
								keyFieldPath = keyFieldPath + *keyFieldList[i].Name + "/{" + *keyFieldList[i].Name + "}"
								if i != len(keyFieldList)-1 {
									keyFieldPath = keyFieldPath + "/"
								}
							}
							methodHttpRuleMap := getFieldMap(val.val.Message())
							var valOpt string = "{ "

							fdHttp := googleApiAnnotationsFiles[0].Extensions().ByName("http")
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
					genServiceProto.Method = append(genServiceProto.Method, genMethodProto)
					genFileComments[methodName] = "Метод " + methodName
				}

				genFileProto.Service = append(genFileProto.Service, genServiceProto)

				genFileProto.MessageType = append(genFileProto.MessageType, entityMessageProtodesc)
			}

			var dpj []*desc.FileDescriptor
			for k, v := range entityFilesJ {
				dpj = append(dpj, v)
				fmt.Println("genFileProto.Dependency", k)
			}
			fmt.Println("genFileProto.Dependency", genFileProto.Dependency)

			genFileDescFd, err := desc.CreateFileDescriptor(genFileProto, dpj...)
			if err != nil {
				panic(err)
			}

			// genFileBuilder, err := builder.FromFile(genFileDescFd)
			// if err != nil {
			// 	panic(err)
			// }

			// fmt.Println("genFileBuilder", genFileBuilder)

			// for i := range genFileDescFd.GetServices() {
			// 	if val, ok := genFileComments[genFileDescFd.GetServices()[i].GetName()]; ok {
			// 		genFileBuilder.GetService(genFileDescFd.GetServices()[i].GetName()).SetComments(builder.Comments{LeadingComment: val})
			// 	}
			// 	for j := range genFileDescFd.GetServices()[i].GetMethods() {
			// 		if val, ok := genFileComments[genFileDescFd.GetServices()[i].GetMethods()[j].GetName()]; ok {
			// 			genFileBuilder.GetService(genFileDescFd.GetServices()[i].GetName()).GetMethod(genFileDescFd.GetServices()[i].GetMethods()[j].GetName()).SetComments(builder.Comments{LeadingComment: val})
			// 		}
			// 	}
			// }

			// for i := range genFileDescFd.GetMessageTypes() {
			// 	if val, ok := genFileComments[genFileDescFd.GetMessageTypes()[i].GetName()]; ok {
			// 		genFileBuilder.GetMessage(genFileDescFd.GetMessageTypes()[i].GetName()).SetComments(builder.Comments{LeadingComment: val})
			// 	}
			// }

			// genFileDesc, err := genFileBuilder.Build()
			// if err != nil {
			// 	panic(err)
			// }

			// fmt.Println("genFileBuilder", genFileDesc.GetDependencies())

			p := new(protoprint.Printer)
			p.SortElements = true
			p.Indent = "    "
			protoStr, err := p.PrintProtoToString(genFileDescFd)
			if err != nil {
				panic(err)
			}
			fmt.Println("print Proto", protoStr)

		}

	}
	fmt.Println("Generation end")
}
