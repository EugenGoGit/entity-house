package template

import (
	"entity-house/internal/comment"
	"entity-house/internal/util"
	"errors"
	"fmt"
	"maps" // Requires Go 1.21+
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	ReplaceFieldOptName       = "replace_field_type_to"
	EntityKeyFieldsOptValue   = "EntityKeyFields"
	FieldCommentsOptName      = "field_comments"
	MessageCommentsOptName    = "message_comments"
	ReplaceMsgTypeNameOptName = "replace_message_type_name_to"
	EnumByMethodAttrOptName   = "enum_by_method_attribute"
	EnumCommentsOptName       = "enum_comments"
	EnumValueCommentsOptName  = "enum_value_comments"
	OneofCommentsOptName      = "oneof_comments"
	UniqueFieldGroupOptName   = "unique_field_group"
)

// GetUniqueFieldGroup возвращает список полей, принадлежащих указанной группе уникальности.
func GetUniqueFieldGroup(entityDesc protoreflect.MessageDescriptor, uniqueFieldGroupEl protoreflect.EnumNumber) []util.FieldFullName {
	m := make(map[protoreflect.EnumNumber][]util.FieldFullName)
	var min protoreflect.EnumNumber = 10000

	for i := 0; i < entityDesc.Fields().Len(); i++ {
		field := entityDesc.Fields().Get(i)
		optsMap := util.GetFieldMap(field.Options().ProtoReflect())
		if val, ok := optsMap[UniqueFieldGroupOptName]; ok {
			groupNum := val.Val.Enum()
			ffn := util.FieldFullName{
				FullName:       string(field.FullName()),
				FieldDescProto: protodesc.ToFieldDescriptorProto(field),
			}
			m[groupNum] = append(m[groupNum], ffn)
			if groupNum < min {
				min = groupNum
			}
		}
	}

	if val, ok := m[uniqueFieldGroupEl]; ok {
		return val
	}
	// Возвращаем минимальную группу, если запрашиваемая не найдена
	return m[min]
}

// GetKeyFields определяет ключевые поля на основе определения.
func GetKeyFields(keyFieldsDefinition *protoreflect.Value, entityDesc protoreflect.MessageDescriptor) []util.FieldFullName {
	if keyFieldsDefinition == nil || keyFieldsDefinition.String() == "<nil>" {
		return GetUniqueFieldGroup(entityDesc, 0)
	}

	defMap := util.GetFieldMap(keyFieldsDefinition.Message())
	if keyFieldListVal, ok := defMap["key_field_list"]; ok {
		listMap := util.GetFieldMap(keyFieldListVal.Val.Message())
		if keyFieldsVal, ok := listMap["key_fields"]; ok {
			var res []util.FieldFullName
			list := keyFieldsVal.Val.List()
			for i := 0; i < list.Len(); i++ {
				fieldName := list.Get(i).String()
				field := entityDesc.Fields().ByName(protoreflect.Name(fieldName))
				if field != nil {
					res = append(res, util.FieldFullName{
						FullName:       string(field.FullName()),
						FieldDescProto: protodesc.ToFieldDescriptorProto(field),
					})
				}
			}
			return res
		}
		// Если key_fields не найдено, возвращаем первую группу
		return GetUniqueFieldGroup(entityDesc, 0)
	}

	if uniqueGroupVal, ok := defMap[UniqueFieldGroupOptName]; ok {
		return GetUniqueFieldGroup(entityDesc, uniqueGroupVal.Val.Enum())
	}

	return GetUniqueFieldGroup(entityDesc, 0)
}

// GetDtoTemplateDesc получает дескриптор шаблона DTO.
func GetDtoTemplateDesc(
	containerFieldName string,
	varFieldMap,
	tmplFieldMap map[string]util.Field,
	varMethod protoreflect.Message,
) (protoreflect.MessageDescriptor, error) {
	var templateDesc protoreflect.MessageDescriptor

	// Берем Шаблон из спецификации в описании сущности
	if val, ok := varFieldMap[containerFieldName]; ok {
		templateFieldMap := util.GetFieldMap(val.Val.Message())
		// Тут только одно поле DTO
		for _, v := range templateFieldMap {
			templateDesc = v.Val.Message().Descriptor()
			break // Выходим после первого (и предположительно единственного)
		}
	}
	// Если Шаблон не задан в спецификации описании сущности, то берем указанный в шаблоне
	if templateDesc == nil {
		if specFieldName, ok := tmplFieldMap[containerFieldName]; ok {
			varContainer := varMethod.Descriptor().Fields().ByName(protoreflect.Name(containerFieldName))
			if varContainer != nil {
				templateDescField := varContainer.Message().Fields().ByName(protoreflect.Name(specFieldName.Val.String()))
				if templateDescField != nil {
					templateDesc = templateDescField.Message()
				} else {
					return nil, fmt.Errorf("в описании спецификации не найдено поле %s.%s.%s", varMethod.Descriptor().Name(), containerFieldName, specFieldName.Val.String())
				}
			} else {
				return nil, fmt.Errorf("в описании спецификации не найдено поле %s", containerFieldName)
			}
		} else {
			return nil, fmt.Errorf("в шаблоне не найдено поле %s", containerFieldName)
		}
	}
	return templateDesc, nil
}

// ApplyTemplate рекурсивно применяет шаблон к дескриптору сообщения.
func ApplyTemplate(
	resultDescProto *descriptorpb.DescriptorProto,
	templatePrefDesc protoreflect.MessageDescriptor,
	entityTypeName string,
	entityKeyFields []util.FieldFullName,
	templateTypeName string,
	methodParameters map[string]util.Field,
	parentNameToType string,
	packageNameToType string,
	parentFullNameToType string,
	fileCommentsMap map[string]string,
	addMessageToProtoRoot map[string]*descriptorpb.DescriptorProto,
	addImportToProtoRoot map[string]protoreflect.FileImport,
) error {

	// --- Обработка полей ---
	for i := 0; i < len(resultDescProto.Field); i++ {
		fieldDescProto := resultDescProto.Field[i]
		fieldTmplPrefDesc := templatePrefDesc.Fields().ByName(protoreflect.Name(*fieldDescProto.Name))
		if fieldTmplPrefDesc == nil {
			continue // Пропускаем поля, которых нет в шаблоне
		}
		fieldTmplOpts := util.GetFieldMap(fieldTmplPrefDesc.Options().ProtoReflect())

		if replaceOpt, ok := fieldTmplOpts[ReplaceFieldOptName]; ok {
			replaceValue := replaceOpt.Val.String()

			if replaceValue == EntityKeyFieldsOptValue {
				// --- Замена на ключевые поля сущности ---
				if len(entityKeyFields) == 0 {
					return fmt.Errorf("не удалось найти ключевые поля сущности %s.%s", packageNameToType, entityTypeName)
				}

				// Копируем первое ключевое поле и заменяем текущее
				firstKeyField := entityKeyFields[0]
				// Копируем прото-дескриптор, чтобы не модифицировать оригинальный
				newFieldDescProto := proto.Clone(firstKeyField.FieldDescProto).(*descriptorpb.FieldDescriptorProto)
				newFieldDescProto.Number = proto.Int32(*fieldDescProto.Number) // Сохраняем номер

				// Копируем опции из шаблона
				for _, opt := range fieldTmplOpts {
					// Пропускаем служебные опции
					if opt.Desc.FullName() == ReplaceFieldOptName || opt.Desc.FullName() == FieldCommentsOptName {
						continue
					}
					newFieldDescProto.GetOptions().ProtoReflect().Set(opt.Desc, opt.Val)
				}
				resultDescProto.Field[i] = newFieldDescProto

				// Обработка комментариев
				if fieldCommentOpt, hasComment := fieldTmplOpts[FieldCommentsOptName]; hasComment {
					commentText := comment.ReplacePlaceholders(fieldCommentOpt.Val.String(), entityTypeName, fileCommentsMap[firstKeyField.FullName], fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
					fileCommentsMap[parentFullNameToType+"."+*newFieldDescProto.Name] = commentText
					newFieldDescProto.GetOptions().ProtoReflect().Clear(fieldCommentOpt.Desc)
				} else {
					fileCommentsMap[parentFullNameToType+"."+*newFieldDescProto.Name] = fileCommentsMap[firstKeyField.FullName]
				}

				// Вставка остальных ключевых полей
				insertOffset := 0
				for j := 1; j < len(entityKeyFields); j++ {
					keyField := entityKeyFields[j]
					clonedKeyFieldDescProto := proto.Clone(keyField.FieldDescProto).(*descriptorpb.FieldDescriptorProto)
					nextNumber := *resultDescProto.Field[i+insertOffset].Number + 1
					clonedKeyFieldDescProto.Number = proto.Int32(nextNumber)

					// Копируем опции
					for _, opt := range fieldTmplOpts {
						if opt.Desc.FullName() == ReplaceFieldOptName || opt.Desc.FullName() == FieldCommentsOptName {
							continue
						}
						clonedKeyFieldDescProto.GetOptions().ProtoReflect().Set(opt.Desc, opt.Val)
					}

					// Вставка в срез
					pos := i + 1 + insertOffset
					resultDescProto.Field = append(resultDescProto.Field, nil)
					copy(resultDescProto.Field[pos+1:], resultDescProto.Field[pos:])
					resultDescProto.Field[pos] = clonedKeyFieldDescProto
					insertOffset++

					// Комментарии для вставленных полей
					if fieldCommentOpt, hasComment := fieldTmplOpts[FieldCommentsOptName]; hasComment {
						commentText := comment.ReplacePlaceholders(fieldCommentOpt.Val.String(), entityTypeName, fileCommentsMap[keyField.FullName], fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
						fileCommentsMap[parentFullNameToType+"."+*clonedKeyFieldDescProto.Name] = commentText
						newFieldDescProto.GetOptions().ProtoReflect().Clear(fieldCommentOpt.Desc)
					} else {
						fileCommentsMap[parentFullNameToType+"."+*clonedKeyFieldDescProto.Name] = fileCommentsMap[keyField.FullName]
					}
				}

				// Убираем опции у последнего вставленного поля
				lastInsertedIndex := i + insertOffset
				lastFieldOpts := util.GetFieldMap(resultDescProto.Field[lastInsertedIndex].Options.ProtoReflect())
				for _, opt := range lastFieldOpts {
					// Убираем опции ключевых полей
					if strings.TrimPrefix(string(opt.Desc.FullName()), "entity.feature.") != string(opt.Desc.FullName()) {
						resultDescProto.Field[lastInsertedIndex].Options.ProtoReflect().Clear(opt.Desc)
					}
					// Убираем опции замены типа
					resultDescProto.Field[lastInsertedIndex].Options.ProtoReflect().Clear(replaceOpt.Desc)
				}

				// Обновляем индекс i, чтобы продолжить со следующего поля после вставленных
				i += insertOffset

				// Проставим номера полей на случай пересечения номеров
				for j := range resultDescProto.Field {
					expectedNumber := int32(j + 1)
					if *resultDescProto.Field[j].Number < expectedNumber {
						resultDescProto.Field[j].Number = proto.Int32(expectedNumber)
					}
				}

				// Уберем опции замены типа
				newFieldDescProto.GetOptions().ProtoReflect().Clear(replaceOpt.Desc)

			} else {
				// --- Замена типа поля на другой тип ---
				replaceToType := comment.ReplacePlaceholders(replaceValue, entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
				fieldDescProto.TypeName = proto.String(replaceToType)

				if *fieldDescProto.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
					fieldTypePrefDesc := fieldTmplPrefDesc.Message()
					fieldTypeOpts := util.GetFieldMap(fieldTypePrefDesc.Options().ProtoReflect())
					msgDescProto := protodesc.ToDescriptorProto(fieldTypePrefDesc)

					// Рекурсивная обработка вложенного сообщения в шаблоне
					if replaceMsgNameOpt, ok := fieldTypeOpts[ReplaceMsgTypeNameOptName]; ok {
						msgDescProto.Options.ProtoReflect().Clear(replaceMsgNameOpt.Desc)
						newMsgName := comment.ReplacePlaceholders(replaceMsgNameOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
						msgDescProto.Name = proto.String(newMsgName)

						err := ApplyTemplate(
							msgDescProto,
							fieldTypePrefDesc,
							entityTypeName,
							entityKeyFields,
							string(fieldTypePrefDesc.FullName()),
							methodParameters,
							newMsgName, // parentNameToType
							packageNameToType,
							packageNameToType+"."+newMsgName, // parentFullNameToType
							fileCommentsMap,
							addMessageToProtoRoot,
							addImportToProtoRoot,
						)
						if err != nil {
							return err
						}
						addMessageToProtoRoot[newMsgName] = msgDescProto
					}

					// Комментарии для вложенного сообщения
					if msgCommentOpt, ok := fieldTypeOpts[MessageCommentsOptName]; ok {
						commentText := comment.ReplacePlaceholders(msgCommentOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
						fileCommentsMap[packageNameToType+"."+msgDescProto.GetName()] = commentText
						msgDescProto.GetOptions().ProtoReflect().Clear(msgCommentOpt.Desc)
					}

					// Комментарии для поля, ссылающегося на это сообщение
					if fieldCommentOpt, ok := util.GetFieldMap(fieldDescProto.Options.ProtoReflect())[FieldCommentsOptName]; ok {
						commentText := comment.ReplacePlaceholders(fieldCommentOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
                        fileCommentsMap[parentFullNameToType+"."+fieldDescProto.GetName()] = commentText
						fieldDescProto.GetOptions().ProtoReflect().Clear(fieldCommentOpt.Desc)
					} else {
						// Если комментарий для поля не задан, берем комментарий от типа
						if typeComment, ok := fileCommentsMap[packageNameToType+"."+fieldDescProto.GetTypeName()]; ok {
							fileCommentsMap[parentFullNameToType+"."+fieldDescProto.GetName()] = typeComment
							if fieldCommentOpt, ok := util.GetFieldMap(fieldDescProto.Options.ProtoReflect())[FieldCommentsOptName]; ok {
								fieldDescProto.GetOptions().ProtoReflect().Clear(fieldCommentOpt.Desc)
							}
						}
					}
				}
			}
			// Убираем опцию replaceOpt
			fieldDescProto.GetOptions().ProtoReflect().Clear(replaceOpt.Desc)
		} else {
			// --- Поле не требует замены типа ---
			if *fieldDescProto.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE ||
				*fieldDescProto.Type == descriptorpb.FieldDescriptorProto_TYPE_ENUM {

				originalTypeName := *fieldDescProto.TypeName
				// Заменяем префикс типа на префикс результата
				newTypeName := strings.ReplaceAll(originalTypeName, "."+templateTypeName+".", parentNameToType+".")

				// Если замены не произошло, проверяем вложенные типы шаблона
				if newTypeName == originalTypeName {
					if *fieldDescProto.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
						// Получаем дескриптор поля из шаблона
						tmplFieldDesc := templatePrefDesc.Fields().Get(i) // Предполагаем, что индекс совпадает
						if tmplFieldDesc != nil && tmplFieldDesc.Message() != nil {
							tmplMsgDesc := tmplFieldDesc.Message()

							// Проверяем, принадлежит ли тип шаблону или импорту
							tmplFilePkg := string(templatePrefDesc.ParentFile().Package())
							fieldMsgFilePkg := string(tmplMsgDesc.ParentFile().Package())

							if tmplFilePkg == fieldMsgFilePkg {
								// Тип из того же файла, что и шаблон - обрабатываем рекурсивно
								msgDescProto := protodesc.ToDescriptorProto(tmplMsgDesc)
								err := ApplyTemplate(
									msgDescProto,
									tmplMsgDesc,
									entityTypeName,
									entityKeyFields,
									string(tmplMsgDesc.FullName()),
									methodParameters,
									string(tmplMsgDesc.Name()),
									packageNameToType,
									string(tmplMsgDesc.ParentFile().Package())+"."+msgDescProto.GetName(),
									fileCommentsMap,
									addMessageToProtoRoot,
									addImportToProtoRoot,
								)
								if err != nil {
									return err
								}

								// Комментарии для добавленного сообщения
								if msgOpts := util.GetFieldMap(msgDescProto.GetOptions().ProtoReflect()); len(msgOpts) > 0 {
									if msgCommentOpt, ok := msgOpts[MessageCommentsOptName]; ok {
										commentText := comment.ReplacePlaceholders(msgCommentOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
										fileCommentsMap[packageNameToType+"."+msgDescProto.GetName()] = commentText
										msgDescProto.GetOptions().ProtoReflect().Clear(msgCommentOpt.Desc)
									}
								}
								addMessageToProtoRoot[msgDescProto.GetName()] = msgDescProto
								newTypeName = "." + packageNameToType + "." + string(tmplMsgDesc.Name()) // Имя в новом файле
							} else {
								// Тип из другого файла - добавляем импорт
								for j := 0; j < templatePrefDesc.ParentFile().Imports().Len(); j++ {
									importDesc := templatePrefDesc.ParentFile().Imports().Get(j)
									if importDesc.Package() == tmplMsgDesc.ParentFile().Package() {
										addImportToProtoRoot[string(importDesc.Package().Name())] = importDesc
										break
									}
								}
							}
						}
					}
				}
				fieldDescProto.TypeName = proto.String(newTypeName)

				// Комментарии для поля
				if fieldCommentOpt, ok := util.GetFieldMap(fieldDescProto.Options.ProtoReflect())[FieldCommentsOptName]; ok {
					commentText := comment.ReplacePlaceholders(fieldCommentOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
					fileCommentsMap[parentFullNameToType+"."+fieldDescProto.GetName()] = commentText
					fieldDescProto.GetOptions().ProtoReflect().Clear(fieldCommentOpt.Desc)
				}
			} else {
				// Скалярный тип - обрабатываем комментарии
				if fieldCommentOpt, ok := util.GetFieldMap(fieldDescProto.Options.ProtoReflect())[FieldCommentsOptName]; ok {
					commentText := comment.ReplacePlaceholders(fieldCommentOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
					fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*fieldDescProto.Name] = commentText
					fieldDescProto.GetOptions().ProtoReflect().Clear(fieldCommentOpt.Desc)
				}
			}
		}
	}

	// --- Обработка Enum'ов ---
	for i := range resultDescProto.EnumType {
		enumDescProto := resultDescProto.EnumType[i]
		enumOpts := util.GetFieldMap(enumDescProto.Options.ProtoReflect())

		// Для enum с опцией enum_by_method_attribute дополняем элементы
		if methodAttrOpt, ok := enumOpts[EnumByMethodAttrOptName]; ok {
			if enumValues, ok := methodParameters[methodAttrOpt.Val.String()]; ok {
				list := enumValues.Val.List()
				for n := 0; n < list.Len(); n++ {
					enumValueMap := util.GetFieldMap(list.Get(n).Message())
					name := enumValueMap["name"].Val.String()
					title := enumValueMap["title"].Val.String()
					number := int32(len(enumDescProto.Value)) // Новый номер
					enumDescProto.Value = append(
						enumDescProto.Value,
						&descriptorpb.EnumValueDescriptorProto{
							Name:   proto.String(name),
							Number: proto.Int32(number),
						})
					commentText := comment.ReplacePlaceholders(title, entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
					fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*enumDescProto.Name+"."+name] = commentText
				}
			}
			enumDescProto.Options.ProtoReflect().Clear(methodAttrOpt.Desc)
		}

		// Комментарии для самого enum'а
		if enumCommentOpt, ok := enumOpts[EnumCommentsOptName]; ok {
			commentText := comment.ReplacePlaceholders(enumCommentOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
			fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*enumDescProto.Name] = commentText
			enumDescProto.Options.ProtoReflect().Clear(enumCommentOpt.Desc)
		}

		// Комментарии для значений enum'а
		for j := range enumDescProto.Value {
			enumValDescProto := enumDescProto.Value[j]
			enumValOpts := util.GetFieldMap(enumValDescProto.Options.ProtoReflect())
			if enumValCommentOpt, ok := enumValOpts[EnumValueCommentsOptName]; ok {
				commentText := comment.ReplacePlaceholders(enumValCommentOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
				fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*enumDescProto.Name+"."+*enumValDescProto.Name] = commentText
				enumValDescProto.Options.ProtoReflect().Clear(enumValCommentOpt.Desc)
			}
		}
	}

	// --- Обработка вложенных сообщений ---
	for i := range resultDescProto.NestedType {
		nestedDescProto := resultDescProto.NestedType[i]
		// Комментарии для вложенного сообщения
		if nestedOpts := util.GetFieldMap(nestedDescProto.GetOptions().ProtoReflect()); len(nestedOpts) > 0 {
			if nestedCommentOpt, ok := nestedOpts[MessageCommentsOptName]; ok {
				commentText := comment.ReplacePlaceholders(nestedCommentOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
				fileCommentsMap[parentFullNameToType+"."+nestedDescProto.GetName()] = commentText
				nestedDescProto.GetOptions().ProtoReflect().Clear(nestedCommentOpt.Desc)
			}
		}

		// Рекурсивный вызов для вложенного типа
		// Находим соответствующий дескриптор в шаблоне
		tmplNestedDesc := templatePrefDesc.Messages().ByName(protoreflect.Name(*nestedDescProto.Name))
		if tmplNestedDesc != nil {
			err := ApplyTemplate(
				nestedDescProto,
				tmplNestedDesc,
				entityTypeName,
				entityKeyFields,
				templateTypeName,
				methodParameters,
				parentNameToType+"."+nestedDescProto.GetName(),
				packageNameToType,
				parentFullNameToType+"."+nestedDescProto.GetName(),
				fileCommentsMap,
				addMessageToProtoRoot,
				addImportToProtoRoot,
			)
			if err != nil {
				return err
			}
		}
	}

	// --- Обработка oneof ---
	for i := range resultDescProto.GetOneofDecl() {
		oneofDescProto := resultDescProto.GetOneofDecl()[i]
		oneofOpts := util.GetFieldMap(oneofDescProto.GetOptions().ProtoReflect())
		if oneofCommentOpt, ok := oneofOpts[OneofCommentsOptName]; ok {
			commentText := comment.ReplacePlaceholders(oneofCommentOpt.Val.String(), entityTypeName, "", fileCommentsMap[packageNameToType+"."+entityTypeName], "method", methodParameters)
			fileCommentsMap[parentFullNameToType+"."+oneofDescProto.GetName()] = commentText
			oneofDescProto.GetOptions().ProtoReflect().Clear(oneofCommentOpt.Desc)
		}
	}

	return nil
}

// GenMethod генерирует метод сервиса на основе шаблона и вариации.
func GenMethod(
	tmplMethod protoreflect.Message, // Опции шаблона метода
	varMethod protoreflect.Message, // Описание вариации метода
	serviceKeyFieldsDefinition protoreflect.Value, // Определение ключевых полей сервиса
	genFileProto *descriptorpb.FileDescriptorProto,
	entityPrefDesc protoreflect.MessageDescriptor,
	entityMessageProtodesc *descriptorpb.DescriptorProto,
	genFileComments map[string]string,
	addMessageToProtoRoot map[string]*descriptorpb.DescriptorProto,
	addImportToProtoRoot map[string]protoreflect.FileImport,
	httpRoot string,
	googleApiAnnotationsPrefDesc protoreflect.FileDescriptor,
	genServiceProto *descriptorpb.ServiceDescriptorProto,
	serviceName string,
) error {

	tmplFieldMap := util.GetFieldMap(tmplMethod)
	varFieldMap := util.GetFieldMap(varMethod)

	// Атрибуты, которые определяются в шаблоне спецификации
	methodParameters := make(map[string]util.Field)
	maps.Copy(methodParameters, tmplFieldMap)
	// Атрибуты, заданные в описании сущности переопределяют те, что заданы в шаблоне
	maps.Copy(methodParameters, varFieldMap)

	requestTemplateDesc, err := GetDtoTemplateDesc("request_template", varFieldMap, tmplFieldMap, varMethod)
	if err != nil {
		return fmt.Errorf("ошибка получения шаблона запроса: %w", err)
	}
	responseTemplateDesc, err := GetDtoTemplateDesc("response_template", varFieldMap, tmplFieldMap, varMethod)
	if err != nil {
		return fmt.Errorf("ошибка получения шаблона ответа: %w", err)
	}

	// Определение уникальных полей сервиса (переопределяется на уровне метода, если есть)
	keyFieldsDefinition := serviceKeyFieldsDefinition
	if keyFieldDefVal, ok := methodParameters["key_fields_definition"]; ok {
		keyFieldsDefinition = keyFieldDefVal.Val
	}

	if methodNameEntry, ok := methodParameters["name"]; !ok {
		return errors.New("отсутствует имя метода в шаблоне для метода " + string(varMethod.Descriptor().FullName()))
	} else {
		methodName := comment.ReplacePlaceholders(methodNameEntry.Val.String(), string(entityPrefDesc.Name()), "", genFileComments[string(entityPrefDesc.ParentFile().Package())+"."+*entityMessageProtodesc.Name], "method", methodParameters)

		// Имя запроса
		requestNameVal, ok := methodParameters["request_name"]
		if !ok || requestNameVal.Val.String() == "<nil>" {
			return errors.New("отсутствует request_name в шаблоне для метода " + string(varMethod.Descriptor().FullName()))
		}
		requestName := comment.ReplacePlaceholders(requestNameVal.Val.String(), string(entityPrefDesc.Name()), "", genFileComments[string(entityPrefDesc.ParentFile().Package())+"."+*entityMessageProtodesc.Name], "method", methodParameters)
		requestFullName := *genFileProto.Package + "." + requestName

		// Имя ответа
		responseNameVal, ok := methodParameters["response_name"]
		if !ok || responseNameVal.Val.String() == "<nil>" {
			return errors.New("отсутствует response_name в шаблоне для метода " + string(varMethod.Descriptor().FullName()))
		}
		responseName := comment.ReplacePlaceholders(responseNameVal.Val.String(), string(entityPrefDesc.Name()), "", genFileComments[string(entityPrefDesc.ParentFile().Package())+"."+*entityMessageProtodesc.Name], "method", methodParameters)
		responseFullName := *genFileProto.Package + "." + responseName

		// Ключевые поля метода
		keyFieldList := GetKeyFields(&keyFieldsDefinition, entityPrefDesc)

		// Генерация запроса
		requestMessageProtodesc := protodesc.ToDescriptorProto(requestTemplateDesc)
		requestMessageProtodesc.Name = proto.String(requestName)
		err = ApplyTemplate(
			requestMessageProtodesc,
			requestTemplateDesc,
			*entityMessageProtodesc.Name,
			keyFieldList,
			string(requestTemplateDesc.FullName()),
			methodParameters,
			requestName,
			string(entityPrefDesc.ParentFile().Package()),
			requestFullName,
			genFileComments,
			addMessageToProtoRoot,
			addImportToProtoRoot,
		)
		if err != nil {
			return fmt.Errorf("ошибка применения шаблона к запросу %s: %w", requestName, err)
		}

		// Генерация ответа
		responseMessageProtodesc := protodesc.ToDescriptorProto(responseTemplateDesc)
		responseMessageProtodesc.Name = proto.String(responseName)
		err = ApplyTemplate(
			responseMessageProtodesc,
			responseTemplateDesc,
			*entityMessageProtodesc.Name,
			keyFieldList,
			string(responseTemplateDesc.FullName()),
			methodParameters,
			responseName,
			string(entityPrefDesc.ParentFile().Package()),
			responseFullName,
			genFileComments,
			addMessageToProtoRoot,
			addImportToProtoRoot,
		)
		if err != nil {
			return fmt.Errorf("ошибка применения шаблона к ответу %s: %w", responseName, err)
		}

		genFileProto.MessageType = append(genFileProto.MessageType, requestMessageProtodesc)
		genFileProto.MessageType = append(genFileProto.MessageType, responseMessageProtodesc)

		// Создание дескриптора метода
		genMethodProto := &descriptorpb.MethodDescriptorProto{
			Name:       proto.String(methodName),
			InputType:  proto.String("." + requestFullName),  // Полное имя с точкой
			OutputType: proto.String("." + responseFullName), // Полное имя с точкой
			Options:    &descriptorpb.MethodOptions{},
		}

		// Потоки
		serverStreaming := false
		if val, ok := methodParameters["server_streaming"]; ok {
			serverStreaming = val.Val.Bool()
		}
		genMethodProto.ServerStreaming = proto.Bool(serverStreaming)

		clientStreaming := false
		if val, ok := methodParameters["client_streaming"]; ok {
			clientStreaming = val.Val.Bool()
		}
		genMethodProto.ClientStreaming = proto.Bool(clientStreaming)

		// HTTP опции
		if httpRuleVal, ok := methodParameters["http_rule"]; ok && httpRoot != "" && httpRoot != "<nil>" {
			keyFieldPrefix := ""
			if prefixVal, ok := methodParameters["key_field_prefix"]; ok {
				keyFieldPrefix = prefixVal.Val.String() + "."
			}

			var keyFieldPath string
			if len(keyFieldList) == 1 {
				keyFieldPath = "{" + keyFieldPrefix + *keyFieldList[0].FieldDescProto.Name + "}"
			} else {
				for i, keyField := range keyFieldList {
					keyFieldPath += *keyField.FieldDescProto.Name + "/{" + keyFieldPrefix + *keyField.FieldDescProto.Name + "}"
					if i != len(keyFieldList)-1 {
						keyFieldPath += "/"
					}
				}
			}

			methodHttpRuleMap := util.GetFieldMap(httpRuleVal.Val.Message())
			fdHttp := googleApiAnnotationsPrefDesc.Extensions().ByName("http")
			if fdHttp != nil && fdHttp.Message() != nil {
				fdHttpV := dynamicpb.NewMessage(fdHttp.Message())
				for k, v := range methodHttpRuleMap {
					fd := fdHttp.Message().Fields().ByName(protoreflect.Name(k))
					if fd != nil {
						fval := strings.ReplaceAll(v.Val.String(), "{HttpRoot}", httpRoot)
						fval = strings.ReplaceAll(fval, "{KeyFields}", keyFieldPath)
                        fval = comment.ReplacePlaceholders(fval, string(entityPrefDesc.Name()), "", genFileComments[string(entityPrefDesc.ParentFile().Package())+"."+*entityMessageProtodesc.Name], "method", methodParameters)
						fval = strings.ToLower(fval)
                        fdHttpV.Set(fd, protoreflect.ValueOf(fval))
					}
				}
				genMethodProto.Options.ProtoReflect().Set(fdHttp, protoreflect.ValueOfMessage(fdHttpV))
			}
		}

		// Комментарии
		var methodComment, requestComment, responseComment string
		if val, ok := methodParameters["leading_comment"]; ok {
			methodComment = comment.ReplacePlaceholders(val.Val.String(), string(entityPrefDesc.Name()), "", genFileComments[string(entityPrefDesc.ParentFile().Package())+"."+*entityMessageProtodesc.Name], "method", methodParameters)
		}
		if val, ok := methodParameters["additional_leading_comment"]; ok {
			methodComment += ".\n" + val.Val.String()
		}
		if val, ok := methodParameters["request_leading_comment"]; ok {
			requestComment = comment.ReplacePlaceholders(val.Val.String(), string(entityPrefDesc.Name()), "", genFileComments[string(entityPrefDesc.ParentFile().Package())+"."+*entityMessageProtodesc.Name], "method", methodParameters)
		}
		if val, ok := methodParameters["response_leading_comment"]; ok {
			responseComment = comment.ReplacePlaceholders(val.Val.String(), string(entityPrefDesc.Name()), "", genFileComments[string(entityPrefDesc.ParentFile().Package())+"."+*entityMessageProtodesc.Name], "method", methodParameters)
		}

		genServiceProto.Method = append(genServiceProto.Method, genMethodProto)
		genFileComments[*genFileProto.Package+"."+serviceName+"."+methodName] = methodComment
		genFileComments[*genFileProto.Package+"."+requestName] = requestComment
		genFileComments[*genFileProto.Package+"."+responseName] = responseComment
	}

	return nil
}

// GenEntityApiSpec генерирует спецификацию API для сущности.
func GenEntityApiSpec(
	apiSpecOpt util.Field,
	entityPrefDesc protoreflect.MessageDescriptor,
	genFileProto *descriptorpb.FileDescriptorProto,
	googleApiAnnotationsPrefDesc protoreflect.FileDescriptor,
	genFileComments map[string]string,
	addMessageToProtoRoot map[string]*descriptorpb.DescriptorProto,
	addImportToProtoRoot map[string]protoreflect.FileImport,
) error {
	// Определяем в каком поле определены шаблоны сервисов
	specTmplOpts := util.GetFieldMap(apiSpecOpt.Val.Message().Descriptor().Options().ProtoReflect())
	// По умолчания это поле service_set
	serviceSetFieldName := "service_set"
	if serviceSetField, ok := specTmplOpts["service_set_field_name"]; ok {
		serviceSetFieldName = serviceSetField.Val.String()
	}
	// Удаляем опцию
	entityPrefDesc.Options().ProtoReflect().Clear(apiSpecOpt.Desc)
	entityMessageProtodesc := protodesc.ToDescriptorProto(entityPrefDesc)

	// Описание сервисов
	apiSpecMap := util.GetFieldMap(apiSpecOpt.Val.Message())
	serviceSetVal, ok := apiSpecMap[serviceSetFieldName]
	if !ok {
		return fmt.Errorf("не указано поле с шаблонами сервисов %s в спецификации %s", serviceSetFieldName, entityPrefDesc.FullName())
	}

	serviceSetMap := util.GetFieldMap(serviceSetVal.Val.Message())

	for _, entitySourceService := range serviceSetMap {
		entityServiceMap := util.GetFieldMap(entitySourceService.Val.Message())

		// Описание сервиса из шаблона
		tmplServiceOptions := entitySourceService.Val.Message().Descriptor().Options().ProtoReflect()
		tmplServiceMap := util.GetFieldMap(tmplServiceOptions)

		// Атрибуты сервиса
		serviceParameters := make(map[string]util.Field)
		maps.Copy(serviceParameters, tmplServiceMap)   // Из шаблона
		maps.Copy(serviceParameters, entityServiceMap) // Из описания (переопределяют)

		serviceName := comment.ReplacePlaceholders(serviceParameters["name"].Val.String(), string(entityPrefDesc.Name()), "", genFileComments[string(entityPrefDesc.ParentFile().Package())+"."+*entityMessageProtodesc.Name], "service", serviceParameters)
		serviceComment := comment.ReplacePlaceholders(serviceParameters["leading_comment"].Val.String(), string(entityPrefDesc.Name()), "", genFileComments[string(entityPrefDesc.ParentFile().Package())+"."+*entityMessageProtodesc.Name], "service", serviceParameters)

		if val, ok := serviceParameters["additional_leading_comment"]; ok {
			serviceComment += ".\n" + val.Val.String()
		}

		// Ключевые поля и HTTP корень
		serviceKeyFieldsDefinition := serviceParameters["key_fields_definition"].Val // Используем определение из сервиса
		httpRoot := serviceParameters["http_root"].Val.String()

		genFileComments[*genFileProto.Package+"."+serviceName] = serviceComment

		// Поиск или создание сервиса
		var genServiceProto *descriptorpb.ServiceDescriptorProto
		serviceIndex := -1
		for i, s := range genFileProto.Service {
			if *s.Name == serviceName {
				genServiceProto = s
				serviceIndex = i
				break
			}
		}
		if genServiceProto == nil {
			genServiceProto = &descriptorpb.ServiceDescriptorProto{Name: proto.String(serviceName)}
		} else {
			// Удаляем существующий сервис из списка, чтобы добавить обновленный
			genFileProto.Service = append(genFileProto.Service[:serviceIndex], genFileProto.Service[serviceIndex+1:]...)
		}

		// Методы
		// Определяем в каком поле определены шаблоны методов
		// По умолчанию это поле method_set
		methodSetFieldName := "method_set"
		if val, ok := serviceParameters["method_set_field_name"]; ok {
			methodSetFieldName = val.Val.String()
		}

		if methodSetVal, ok := entityServiceMap[methodSetFieldName]; ok {
			requiredMethods := util.GetFieldMap(methodSetVal.Val.Message())

			for _, method := range requiredMethods {
				if method.Desc.IsList() {
					list := method.Val.List()
					for j := 0; j < list.Len(); j++ {
						err := GenMethod(
							list.Get(j).Message().Descriptor().Options().ProtoReflect(), // Опции шаблона метода
							list.Get(j).Message(), // Вариация метода
							serviceKeyFieldsDefinition,
							genFileProto,
							entityPrefDesc,
							entityMessageProtodesc,
							genFileComments,
							addMessageToProtoRoot,
							addImportToProtoRoot,
							httpRoot,
							googleApiAnnotationsPrefDesc,
							genServiceProto,
							serviceName,
						)
						if err != nil {
							return fmt.Errorf("ошибка генерации метода из списка: %w", err)
						}
					}
				} else {
					err := GenMethod(
						method.Desc.Message().Options().ProtoReflect(), // Опции шаблона метода
						method.Val.Message(),                           // Вариация метода
						serviceKeyFieldsDefinition,
						genFileProto,
						entityPrefDesc,
						entityMessageProtodesc,
						genFileComments,
						addMessageToProtoRoot,
						addImportToProtoRoot,
						httpRoot,
						googleApiAnnotationsPrefDesc,
						genServiceProto,
						serviceName,
					)
					if err != nil {
						return fmt.Errorf("ошибка генерации метода: %w", err)
					}
				}
			}
		}

		// Добавляем (обновленный) сервис обратно
		genFileProto.Service = append(genFileProto.Service, genServiceProto)
	}

	// Добавляем саму сущность
	genFileProto.MessageType = append(genFileProto.MessageType, entityMessageProtodesc)
	return nil
}
