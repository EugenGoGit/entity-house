package generator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/builder"
	"google.golang.org/protobuf/proto"
	// "github.com/jhump/protoreflect/desc/protoprint"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"entity-house/comment"  // Замените "your_project_name"
	"entity-house/printer"  // Замените "your_project_name"
	"entity-house/template" // Замените "your_project_name"
	"entity-house/util"     // Замените "your_project_name"
)

const (
	ApiSpecificationOptFullName = "entity.feature.api.specification"
	AmlSpecificationOptFullName = "entity.feature.aml.specification"
	FeatureOptionPrefix         = "entity.feature."
)

// getProtoJ рекурсивно создает FileDescriptor из proto, разрешая зависимости.
// appendTo используется для кэширования уже созданных дескрипторов.
func getProtoJ(c protocompile.Compiler, fd *descriptorpb.FileDescriptorProto, appendTo map[string]*desc.FileDescriptor) *desc.FileDescriptor {
	var deps []*desc.FileDescriptor
	for _, depName := range fd.Dependency {
		if existingDep, ok := appendTo[depName]; ok {
			deps = append(deps, existingDep)
		} else {
			compiledDeps, err := c.Compile(context.Background(), depName)
			if err != nil {
				log.Printf("Warning: Failed to compile dependency %s: %v", depName, err)
				// Продолжаем, надеясь, что desc.CreateFileDescriptor справится
				continue
			}
			if len(compiledDeps) > 0 {
				depProto := protodesc.ToFileDescriptorProto(compiledDeps[0])
				depJhump := getProtoJ(c, depProto, appendTo)
				deps = append(deps, depJhump)
				appendTo[depName] = depJhump
			}
		}
	}
	res, err := desc.CreateFileDescriptor(fd, deps...)
	if err != nil {
		log.Fatalf("Failed to create FileDescriptor for %s: %v", fd.GetName(), err)
	}
	return res
}

// BuildEntityFeatures основная функция генерации.
func BuildEntityFeatures(entityFilePath string, importPaths []string) (map[string]string, error) {
	var protoFileNames []string
	errWalk := filepath.Walk(entityFilePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".proto" {
			protoFileNames = append(protoFileNames, filepath.ToSlash(path)) // Normalize path separators
		}
		return nil
	})
	if errWalk != nil {
		return nil, fmt.Errorf("error walking path %s: %w", entityFilePath, errWalk)
	}

	if len(protoFileNames) == 0 {
		return nil, errors.New("no .proto files found in the specified path")
	}

	var warningErrorsWithPos []reporter.ErrorWithPos
	rep := reporter.NewReporter(
		func(errorWithPos reporter.ErrorWithPos) error {
			log.Printf("Protocompile Error: %v", errorWithPos)
			return nil // Не прерываем компиляцию
		},
		func(errorWithPos reporter.ErrorWithPos) {
			log.Printf("Protocompile Warning: %v", errorWithPos)
			warningErrorsWithPos = append(warningErrorsWithPos, errorWithPos)
		},
	)

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: importPaths,
		}),
		Reporter:       rep,
		SourceInfoMode: protocompile.SourceInfoStandard,
		RetainASTs:     true,
	}

	sourceFilePrefDescs, err := compiler.Compile(context.Background(), protoFileNames...)
	if err != nil {
		return nil, fmt.Errorf("protocompile failed: %w", err)
	}

	googleApiAnnotationsFiles, err := compiler.Compile(context.Background(), "google/api/annotations.proto")
	if err != nil {
		log.Printf("Warning: Could not compile google/api/annotations.proto, HTTP options will not be added: %v", err)
		// Не прерываем, просто не будем добавлять аннотации
	}

	filesJhumpDesc := make(map[string]*desc.FileDescriptor)
	result := make(map[string]string)

	for _, sourceFilePrefDesc := range sourceFilePrefDescs {
		entityMsgApiSpecOpt := make(map[protoreflect.MessageDescriptor]util.Field)
		entityMsgAmlSpecOpt := make(map[protoreflect.MessageDescriptor]util.Field)

		// Собираем сущности с опциями
		for i := 0; i < sourceFilePrefDesc.Messages().Len(); i++ {
			msgDesc := sourceFilePrefDesc.Messages().Get(i)
			for _, msgOpt := range util.GetFieldMap(msgDesc.Options().ProtoReflect()) {
				msgOptOpts := util.GetFieldMap(msgOpt.Desc.Options().ProtoReflect())
				// API Specification
				if specOpt, ok := msgOptOpts["specification"]; ok {
					if string(specOpt.FullName) == ApiSpecificationOptFullName {
						entityMsgApiSpecOpt[msgDesc] = msgOpt
					}
				}
				// AML Specification
				if string(msgOpt.FullName) == AmlSpecificationOptFullName {
					entityMsgAmlSpecOpt[msgDesc] = msgOpt
				}
			}
		}

		// Обработка AML (заглушка)
		if len(entityMsgAmlSpecOpt) > 0 {
			for entityPrefDesc := range entityMsgAmlSpecOpt {
				log.Printf("Warning: %s: entity.feature.aml_specification not implemented", entityPrefDesc.FullName())
			}
		}

		// Обработка API Specification
		if len(entityMsgApiSpecOpt) > 0 {
			sourceFileDescriptorProto := protodesc.ToFileDescriptorProto(sourceFilePrefDesc)
			sourceFileJhumpDesc := getProtoJ(compiler, sourceFileDescriptorProto, filesJhumpDesc)

			addMessageToProtoRoot := make(map[string]*descriptorpb.DescriptorProto)
			addImportToProtoRoot := make(map[string]protoreflect.FileImport)
			genFileComments := make(map[string]string)
			leadingFileComments := make(map[string][]string) // Для комментариев файла

			// Создаем новый файл для генерации
			genFileProto := &descriptorpb.FileDescriptorProto{
				Syntax:     proto.String("proto3"),
				Name:       proto.String(string(sourceFilePrefDesc.FullName())),
				Package:    proto.String(string(sourceFilePrefDesc.Package())),
				Dependency: sourceFileDescriptorProto.Dependency,
				Options:    sourceFileDescriptorProto.Options, // Копируем опции
			}

			// Типы без фичей specification добавим в генерацию
			for i := 0; i < sourceFilePrefDesc.Messages().Len(); i++ {
				msgDesc := sourceFilePrefDesc.Messages().Get(i)
				if _, hasApiSpec := entityMsgApiSpecOpt[msgDesc]; !hasApiSpec {
					genFileProto.MessageType = append(genFileProto.MessageType, protodesc.ToDescriptorProto(msgDesc))
				}
			}

			// Существующие сервисы и их комментарии
			for i := 0; i < sourceFilePrefDesc.Services().Len(); i++ {
				serviceDesc := sourceFilePrefDesc.Services().Get(i)
				genFileProto.Service = append(genFileProto.Service, protodesc.ToServiceDescriptorProto(serviceDesc))
				serviceJhump := sourceFileJhumpDesc.FindService(string(serviceDesc.FullName()))
				if serviceJhump != nil && serviceJhump.GetSourceInfo().LeadingComments != nil {
					genFileComments[string(serviceDesc.FullName())] = *serviceJhump.GetSourceInfo().LeadingComments
				}
				for j := 0; j < serviceDesc.Methods().Len(); j++ {
					methodDesc := serviceDesc.Methods().Get(j)
					if serviceJhump != nil {
						methodJhump := serviceJhump.FindMethodByName(string(methodDesc.Name()))
						if methodJhump != nil && methodJhump.GetSourceInfo().LeadingComments != nil {
							genFileComments[string(methodDesc.FullName())] = *methodJhump.GetSourceInfo().LeadingComments
						}
					}
				}
			}

			// Enums и их комментарии
			for i := 0; i < sourceFilePrefDesc.Enums().Len(); i++ {
				enumDesc := sourceFilePrefDesc.Enums().Get(i)
				genFileProto.EnumType = append(genFileProto.EnumType, protodesc.ToEnumDescriptorProto(enumDesc))
				enumJhump := sourceFileJhumpDesc.FindEnum(string(enumDesc.FullName()))
				if enumJhump != nil && enumJhump.GetSourceInfo().LeadingComments != nil {
					genFileComments[string(enumDesc.FullName())] = *enumJhump.GetSourceInfo().LeadingComments
				}
				for j := 0; j < enumDesc.Values().Len(); j++ {
					enumValueDesc := enumDesc.Values().Get(j)
					if enumJhump != nil {
						enumValueJhump := enumJhump.FindValueByName(string(enumValueDesc.Name()))
						if enumValueJhump != nil && enumValueJhump.GetSourceInfo().LeadingComments != nil {
							genFileComments[enumValueJhump.GetFullyQualifiedName()] = *enumValueJhump.GetSourceInfo().LeadingComments
						}
					}
				}
			}

			// Обработка опций файла (замена плейсхолдеров)
			for _, v := range util.GetFieldMap(sourceFilePrefDesc.Options().ProtoReflect()) {
				if v.Desc.Kind() == protoreflect.StringKind {
					varStr := genFileProto.Options.ProtoReflect().Get(v.Desc).String()
					varStr = strings.ReplaceAll(varStr, "{PackageName}", string(sourceFilePrefDesc.Package()))
					varStr = strings.ReplaceAll(varStr, "{PackageNameDotCamelCase}", util.ToCamelCase(string(sourceFilePrefDesc.Package()), ".", "."))
					varStr = strings.ReplaceAll(varStr, "{PackageNameSnakeCase}", util.ToSnakeCase(string(sourceFilePrefDesc.Package()), "."))
					varStr = strings.ReplaceAll(varStr, "{PackageNameCamelCase}", util.ToCamelCase(string(sourceFilePrefDesc.Package()), ".", ""))
					varStr = strings.ReplaceAll(varStr, "{PackageNameUpperCase}", strings.ToUpper(strings.ReplaceAll(string(sourceFilePrefDesc.Package()), ".", "")))
					varStr = strings.ReplaceAll(varStr, "{PackageNameDoubleSlashCamelCase}", util.ToCamelCase(string(sourceFilePrefDesc.Package()), ".", "\\"))
					varStr = strings.ReplaceAll(varStr, "{PackageNameDoubleColonCamelCase}", util.ToCamelCase(string(sourceFilePrefDesc.Package()), ".", "::"))
					genFileProto.Options.ProtoReflect().Set(v.Desc, protoreflect.ValueOf(varStr))
				}
			}

			// Заполним комментарии по всем типам исходного файла
			for i := 0; i < len(sourceFileJhumpDesc.GetMessageTypes()); i++ {
				comment.GetMessageComments(sourceFileJhumpDesc.GetMessageTypes()[i], genFileComments)
			}
			for i := 0; i < len(sourceFileJhumpDesc.GetServices()); i++ {
				service := sourceFileJhumpDesc.GetServices()[i]
				if service.GetSourceInfo().LeadingComments != nil {
					genFileComments[service.GetFullyQualifiedName()] = *service.GetSourceInfo().LeadingComments
				}
			}

			// Обрабатываем сущности с API Specification
			for entityPrefDesc, apiSpecOpt := range entityMsgApiSpecOpt {
				// Комментарий файла из спецификации
				apiSpecMap := util.GetFieldMap(apiSpecOpt.Val.Message())
				if leadCommSpec, ok := apiSpecMap["leading_comment"]; ok {
					commentText := strings.ReplaceAll(leadCommSpec.Val.String(), "{EntityTypeName}", string(entityPrefDesc.Name()))
					leadingFileComments[sourceFileJhumpDesc.GetFile().GetName()] = append(leadingFileComments[sourceFileJhumpDesc.GetFile().GetName()], commentText)
				} else {
					// Комментарий из шаблона спецификации
					specTmplOpts := util.GetFieldMap(apiSpecOpt.Val.Message().Descriptor().Options().ProtoReflect())
					if specTmpl, ok := specTmplOpts["specification_tmpl"]; ok {
						specTmplMap := util.GetFieldMap(specTmpl.Val.Message())
						if leadCommSpecTmpl, ok := specTmplMap["leading_comment"]; ok {
							commentText := strings.ReplaceAll(leadCommSpecTmpl.Val.String(), "{EntityTypeName}", string(entityPrefDesc.Name()))
							leadingFileComments[sourceFileJhumpDesc.GetFile().GetName()] = append(leadingFileComments[sourceFileJhumpDesc.GetFile().GetName()], commentText)
						}
					} else {
						// log.Printf("Warning: Не указан specification_tmpl в шаблоне %s", entityPrefDesc.FullName())
						// Не критично, продолжаем
					}
				}

				// Генерация API для сущности
				var annotationsFileDesc protoreflect.FileDescriptor
				if len(googleApiAnnotationsFiles) > 0 {
					annotationsFileDesc = googleApiAnnotationsFiles[0]
				}
				errGen := template.GenEntityApiSpec(
					apiSpecOpt,
					entityPrefDesc,
					genFileProto,
					annotationsFileDesc,
					genFileComments,
					addMessageToProtoRoot,
					addImportToProtoRoot,
				)
				if errGen != nil {
					return nil, fmt.Errorf("failed to generate API spec for %s: %w", entityPrefDesc.FullName(), errGen)
				}
			}

			// Убираем опции entity.feature с полей в сгенерированном файле
			for _, msg := range genFileProto.MessageType {
				for _, field := range msg.Field {
					fieldOptsMap := util.GetFieldMap(field.Options.ProtoReflect())
					for _, opt := range fieldOptsMap {
						if strings.Contains(string(opt.FullName), FeatureOptionPrefix) {
							field.Options.ProtoReflect().Clear(opt.Desc)
						}
					}
				}
			}

			// Добавим типы, которые пришли с шаблонами
			for _, tmplMsg := range addMessageToProtoRoot {
				// Проверка на дубликаты
				found := false
				for i, existingMsg := range genFileProto.MessageType {
					if *existingMsg.Name == *tmplMsg.Name {
						log.Printf("Warning: Тип %s.%s уже присутствует в генерации, он будет заменен", *genFileProto.Name, *tmplMsg.Name)
						genFileProto.MessageType[i] = tmplMsg
						found = true
						break
					}
				}
				if !found {
					genFileProto.MessageType = append(genFileProto.MessageType, tmplMsg)
				}
			}

			// Нормализуем и обновляем импорты
			deps := make(map[string]string)
			for _, dep := range genFileProto.Dependency {
				deps[dep] = dep
			}
			fmt.Println("genFileProto.Dependency", genFileProto.Dependency, deps)
			delete(deps, "entity_feature/feature.proto")
			// Убираем импорт спецификации (если он был)
			for _, apiSpecOpt := range entityMsgApiSpecOpt {
				specPath := apiSpecOpt.Desc.ParentFile().Path()
				delete(deps, specPath)
			}
			// Добавим импорты из шаблонов
			for _, imp := range addImportToProtoRoot {
				deps[imp.Path()] = imp.Path()
			}
			// Уберем импорты спецификации снова (на всякий случай)
			for _, apiSpecOpt := range entityMsgApiSpecOpt {
				specPath := apiSpecOpt.Desc.ParentFile().Path()
				delete(deps, specPath)
			}

			var depsArr []string
			for dep := range deps {
				depsArr = append(depsArr, dep)
			}
			genFileProto.Dependency = depsArr

			// Создаем дескриптор для генерации
			var depJhumpDescs []*desc.FileDescriptor
			for _, depJhump := range filesJhumpDesc {
				depJhumpDescs = append(depJhumpDescs, depJhump)
			}
			genFileJhumpDesc, errCreate := desc.CreateFileDescriptor(genFileProto, depJhumpDescs...)
			if errCreate != nil {
				return nil, fmt.Errorf("failed to create generated FileDescriptor: %w", errCreate)
			}

			// Создаем builder
			genFileBuilder, errFrom := builder.FromFile(genFileJhumpDesc)
			if errFrom != nil {
				return nil, fmt.Errorf("failed to create builder from FileDescriptor: %w", errFrom)
			}

			// Заполняем комментарии
			for i := 0; i < len(genFileJhumpDesc.GetServices()); i++ {
				serviceDesc := genFileJhumpDesc.GetServices()[i]
				if commentText, ok := genFileComments[serviceDesc.GetFullyQualifiedName()]; ok {
					serviceBuilder := genFileBuilder.GetService(serviceDesc.GetName())
					if serviceBuilder != nil {
						serviceBuilder.SetComments(builder.Comments{LeadingComment: commentText})
					}
				}
				for j := 0; j < len(serviceDesc.GetMethods()); j++ {
					methodDesc := serviceDesc.GetMethods()[j]
					if commentText, ok := genFileComments[methodDesc.GetFullyQualifiedName()]; ok {
						serviceBuilder := genFileBuilder.GetService(serviceDesc.GetName())
						if serviceBuilder != nil {
							methodBuilder := serviceBuilder.GetMethod(methodDesc.GetName())
							if methodBuilder != nil {
								methodBuilder.SetComments(builder.Comments{LeadingComment: commentText})
							}
						}
					}
				}
			}

			for i := 0; i < len(genFileJhumpDesc.GetMessageTypes()); i++ {
				msgDesc := genFileJhumpDesc.GetMessageTypes()[i]
				msgBuilder := genFileBuilder.GetMessage(msgDesc.GetName())
				if msgBuilder != nil {
					comment.FillMessageComments(msgDesc, msgBuilder, genFileComments)
				}
			}

			for i := 0; i < len(genFileJhumpDesc.GetEnumTypes()); i++ {
				enumDesc := genFileJhumpDesc.GetEnumTypes()[i]
				if commentText, ok := genFileComments[enumDesc.GetFullyQualifiedName()]; ok {
					enumBuilder := genFileBuilder.GetEnum(enumDesc.GetName())
					if enumBuilder != nil {
						enumBuilder.SetComments(builder.Comments{LeadingComment: commentText})
					}
				}
				for j := 0; j < len(enumDesc.GetValues()); j++ {
					enumValueDesc := enumDesc.GetValues()[j]
					if commentText, ok := genFileComments[enumValueDesc.GetFullyQualifiedName()]; ok {
						enumBuilder := genFileBuilder.GetEnum(enumDesc.GetName())
						if enumBuilder != nil {
							enumValueBuilder := enumBuilder.GetValue(enumValueDesc.GetName())
							if enumValueBuilder != nil {
								enumValueBuilder.SetComments(builder.Comments{LeadingComment: commentText})
							}
						}
					}
				}
			}

			// Строим финальный дескриптор
			genFileDesc, errBuild := genFileBuilder.Build()
			if errBuild != nil {
				return nil, fmt.Errorf("failed to build final FileDescriptor: %w", errBuild)
			}

			// Печатаем в строку
			p := printer.CreatePrinter()
			protoStr, errPrint := p.PrintProtoToString(genFileDesc)
			if errPrint != nil {
				return nil, fmt.Errorf("failed to print proto to string: %w", errPrint)
			}

			// Добавляем комментарии файла
			finalProtoStr := printer.AddLeadingComments(protoStr, leadingFileComments[sourceFileJhumpDesc.GetFile().GetName()])

			result[sourceFileJhumpDesc.GetFile().GetName()] = finalProtoStr
		}
	}

	return result, nil
}
