package template

import (
    "errors"
    "fmt"
    "strings"
    "maps" // Requires Go 1.21+

    // "github.com/jhump/protoreflect/desc"
    // "github.com/jhump/protoreflect/desc/protoprint"
    "google.golang.org/protobuf/reflect/protodesc"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/descriptorpb"
    "google.golang.org/protobuf/types/dynamicpb"
    "google.golang.org/protobuf/proto"

    "entity-house/comment" // Замените "your_project_name"
    "entity-house/util"    // Замените "your_project_name"
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
    methodDescFields map[string]util.Field,
    parentNameToType string,
    packageNameToType string,
    parentFullNameToType string,
    fileCommentsMap map[string]string,
    addMessageToProtoRoot map[string]*descriptorpb.DescriptorProto,
    addImportToProtoRoot map[string]protoreflect.FileImport,
    linkedTypeName string,
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
                    commentText := comment.ReplacePlaceholdersWithKeyField(fieldCommentOpt.Val.String(), fileCommentsMap[firstKeyField.FullName], entityTypeName)
                    fileCommentsMap[parentFullNameToType+"."+*newFieldDescProto.Name] = commentText
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
                        commentText := comment.ReplacePlaceholdersWithKeyField(fieldCommentOpt.Val.String(), fileCommentsMap[keyField.FullName], entityTypeName)
                        fileCommentsMap[parentFullNameToType+"."+*clonedKeyFieldDescProto.Name] = commentText
                    } else {
                        fileCommentsMap[parentFullNameToType+"."+*clonedKeyFieldDescProto.Name] = fileCommentsMap[keyField.FullName]
                    }
                }

                // Убираем опции ключевых полей у последнего вставленного поля
                lastInsertedIndex := i + insertOffset
                lastFieldOpts := util.GetFieldMap(resultDescProto.Field[lastInsertedIndex].Options.ProtoReflect())
                for _, opt := range lastFieldOpts {

                    fmt.Println("opt.Desc.FullName()", opt.Desc.FullName(), opt.Desc.Name())
                    if strings.TrimPrefix(string(opt.Desc.FullName()),"entity.feature.") != string(opt.Desc.FullName()) {
                        resultDescProto.Field[lastInsertedIndex].Options.ProtoReflect().Clear(opt.Desc)
                    }
                }

                // Обновляем индекс i, чтобы продолжить со следующего поля после вставленных
                i += insertOffset

                // Проставим номера полей на случай пересечения номеров (опционально, можно улучшить)
                for j := range resultDescProto.Field {
                    expectedNumber := int32(j + 1)
                    if *resultDescProto.Field[j].Number < expectedNumber {
                        resultDescProto.Field[j].Number = proto.Int32(expectedNumber)
                    }
                }

            } else {
                // --- Замена типа поля на другой тип ---
                replaceToType := comment.ReplacePlaceholders(replaceValue, entityTypeName, linkedTypeName)
                fieldDescProto.TypeName = proto.String(replaceToType)

                if *fieldDescProto.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
                    fieldTypePrefDesc := fieldTmplPrefDesc.Message()
                    fieldTypeOpts := util.GetFieldMap(fieldTypePrefDesc.Options().ProtoReflect())
                    msgDescProto := protodesc.ToDescriptorProto(fieldTypePrefDesc)

                    // Рекурсивная обработка вложенного сообщения в шаблоне
                    if replaceMsgNameOpt, ok := fieldTypeOpts[ReplaceMsgTypeNameOptName]; ok {
                        msgDescProto.Options.ProtoReflect().Clear(replaceMsgNameOpt.Desc)
                        newMsgName := comment.ReplacePlaceholders(replaceMsgNameOpt.Val.String(), entityTypeName, linkedTypeName)
                        msgDescProto.Name = proto.String(newMsgName)

                        err := ApplyTemplate(
                            msgDescProto,
                            fieldTypePrefDesc,
                            entityTypeName,
                            entityKeyFields,
                            string(fieldTypePrefDesc.FullName()),
                            methodDescFields,
                            newMsgName, // parentNameToType
                            packageNameToType,
                            packageNameToType+"."+newMsgName, // parentFullNameToType
                            fileCommentsMap,
                            addMessageToProtoRoot,
                            addImportToProtoRoot,
                            linkedTypeName,
                        )
                        if err != nil {
                            return err
                        }
                        addMessageToProtoRoot[newMsgName] = msgDescProto
                    }

                    // Комментарии для вложенного сообщения
                    if msgCommentOpt, ok := fieldTypeOpts[MessageCommentsOptName]; ok {
                        commentText := comment.ReplacePlaceholdersWithComments(msgCommentOpt.Val.String(), fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
                        fileCommentsMap[packageNameToType+"."+msgDescProto.GetName()] = commentText
                        msgDescProto.GetOptions().ProtoReflect().Clear(msgCommentOpt.Desc)
                    }

                    // Комментарии для поля, ссылающегося на это сообщение
                    if fieldCommentOpt, ok := util.GetFieldMap(fieldDescProto.Options.ProtoReflect())[FieldCommentsOptName]; ok {
                        commentText := comment.ReplacePlaceholdersWithComments(fieldCommentOpt.Val.String(), fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
                        commentText = strings.ReplaceAll(commentText, "{EntityTypeName}", entityTypeName)
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
            // Убираем опцию replace_field_type_to
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
                                    methodDescFields,
                                    string(tmplMsgDesc.Name()),
                                    packageNameToType,
                                    string(tmplMsgDesc.ParentFile().Package())+"."+msgDescProto.GetName(),
                                    fileCommentsMap,
                                    addMessageToProtoRoot,
                                    addImportToProtoRoot,
                                    linkedTypeName,
                                )
                                if err != nil {
                                    return err
                                }

                                // Комментарии для добавленного сообщения
                                if msgOpts := util.GetFieldMap(msgDescProto.GetOptions().ProtoReflect()); len(msgOpts) > 0 {
                                    if msgCommentOpt, ok := msgOpts[MessageCommentsOptName]; ok {
                                        commentText := comment.ReplacePlaceholdersWithComments(msgCommentOpt.Val.String(), fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
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
                    commentText := comment.ReplacePlaceholdersWithComments(fieldCommentOpt.Val.String(), fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
                    fileCommentsMap[parentFullNameToType+"."+fieldDescProto.GetName()] = commentText
                    fieldDescProto.GetOptions().ProtoReflect().Clear(fieldCommentOpt.Desc)
                }
            } else {
                // Скалярный тип - обрабатываем комментарии
                if fieldCommentOpt, ok := util.GetFieldMap(fieldDescProto.Options.ProtoReflect())[FieldCommentsOptName]; ok {
                    commentText := comment.ReplacePlaceholdersWithComments(fieldCommentOpt.Val.String(), fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
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
            if enumValues, ok := methodDescFields[methodAttrOpt.Val.String()]; ok {
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
                    commentText := comment.ReplacePlaceholdersWithComments(title, fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
                    fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*enumDescProto.Name+"."+name] = commentText
                }
            }
            enumDescProto.Options.ProtoReflect().Clear(methodAttrOpt.Desc)
        }

        // Комментарии для самого enum'а
        if enumCommentOpt, ok := enumOpts[EnumCommentsOptName]; ok {
            commentText := comment.ReplacePlaceholdersWithComments(enumCommentOpt.Val.String(), fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
            fileCommentsMap[packageNameToType+"."+parentNameToType+"."+*enumDescProto.Name] = commentText
            enumDescProto.Options.ProtoReflect().Clear(enumCommentOpt.Desc)
        }

        // Комментарии для значений enum'а
        for j := range enumDescProto.Value {
            enumValDescProto := enumDescProto.Value[j]
            enumValOpts := util.GetFieldMap(enumValDescProto.Options.ProtoReflect())
            if enumValCommentOpt, ok := enumValOpts[EnumValueCommentsOptName]; ok {
                commentText := comment.ReplacePlaceholdersWithComments(enumValCommentOpt.Val.String(), fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
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
                commentText := comment.ReplacePlaceholdersWithComments(nestedCommentOpt.Val.String(), fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
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
                methodDescFields,
                parentNameToType+"."+nestedDescProto.GetName(),
                packageNameToType,
                parentFullNameToType+"."+nestedDescProto.GetName(),
                fileCommentsMap,
                addMessageToProtoRoot,
                addImportToProtoRoot,
                linkedTypeName,
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
            commentText := comment.ReplacePlaceholdersWithComments(oneofCommentOpt.Val.String(), fileCommentsMap[packageNameToType+"."+entityTypeName], linkedTypeName)
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

    tmplFieldMap := util.GetFieldMap(util.GetFieldMap(tmplMethod)["method_tmpl"].Val.Message())
    varFieldMap := util.GetFieldMap(varMethod)

    // Атрибуты, которые определяются в шаблоне спецификации
    methodParameters := make(map[string]util.Field)
    if tmplAttrs, ok := tmplFieldMap["attributes"]; ok {
        maps.Copy(methodParameters, util.GetFieldMap(tmplAttrs.Val.Message()))
    }
    // Атрибуты, заданные в описании сущности переопределяют те, что заданы в шаблоне
    maps.Copy(methodParameters, varFieldMap)

    if _, ok := methodParameters["comments"]; !ok {
        return errors.New("отсутствует атрибуты шаблона с умолчаниями комментариев для метода " + string(varMethod.Descriptor().FullName()))
    }
    commentsMap := util.GetFieldMap(methodParameters["comments"].Val.Message())
    maps.Copy(methodParameters, commentsMap) // Переопределяем комментарии

    requestTemplateDesc, err := GetDtoTemplateDesc("request_template", varFieldMap, tmplFieldMap, varMethod)
    if err != nil {
        return fmt.Errorf("ошибка получения шаблона запроса: %w", err)
    }
    responseTemplateDesc, err := GetDtoTemplateDesc("response_template", varFieldMap, tmplFieldMap, varMethod)
    if err != nil {
        return fmt.Errorf("ошибка получения шаблона ответа: %w", err)
    }

    var linkedTypeName, linkedTypeKeyFieldPath string
    if linkedTypeVal, ok := methodParameters["linked_type"]; ok {
        linkedTypeMap := util.GetFieldMap(linkedTypeVal.Val.Message())
        if nameVal, ok := linkedTypeMap["name"]; ok {
            linkedTypeName = nameVal.Val.String()
        }
        if keyPathVal, ok := linkedTypeMap["key_field_path"]; ok {
            linkedTypeKeyFieldPath = keyPathVal.Val.String()
        }
    }

    // Определение уникальных полей сервиса (переопределяется на уровне метода, если есть)
    keyFieldsDefinition := serviceKeyFieldsDefinition
    if keyFieldDefVal, ok := methodParameters["key_fields_definition"]; ok {
        keyFieldsDefinition = keyFieldDefVal.Val
    }

    if methodNameEntry, ok := methodParameters["name"]; !ok {
        return errors.New("отсутствует имя метода в шаблоне для метода " + string(varMethod.Descriptor().FullName()))
    } else {
        methodName := comment.ReplacePlaceholders(methodNameEntry.Val.String(), string(entityPrefDesc.Name()), linkedTypeName)

        // Имя запроса
        requestNameVal, ok := methodParameters["request_name"]
        if !ok || requestNameVal.Val.String() == "<nil>" {
            return errors.New("отсутствует request_name в шаблоне для метода " + string(varMethod.Descriptor().FullName()))
        }
        requestName := comment.ReplacePlaceholders(requestNameVal.Val.String(), string(entityPrefDesc.Name()), linkedTypeName)
        requestFullName := *genFileProto.Package + "." + requestName

        // Имя ответа
        responseNameVal, ok := methodParameters["response_name"]
        if !ok || responseNameVal.Val.String() == "<nil>" {
            return errors.New("отсутствует response_name в шаблоне для метода " + string(varMethod.Descriptor().FullName()))
        }
        responseName := comment.ReplacePlaceholders(responseNameVal.Val.String(), string(entityPrefDesc.Name()), linkedTypeName)
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
            linkedTypeName,
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
            linkedTypeName,
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
                        httpPath := strings.ReplaceAll(v.Val.String(), "{HttpRoot}", httpRoot)
                        httpPath = strings.ReplaceAll(httpPath, "{KeyFields}", keyFieldPath)
                        httpPath = strings.ReplaceAll(httpPath, "{LinkKeyFieldPath}", linkedTypeKeyFieldPath)
                        httpPath = strings.ReplaceAll(httpPath, "{LinkedTypeName}", strings.ToLower(linkedTypeName))

                        fdHttpV.Set(fd, protoreflect.ValueOf(httpPath))
                    }
                }
                genMethodProto.Options.ProtoReflect().Set(fdHttp, protoreflect.ValueOfMessage(fdHttpV.ProtoReflect()))
            }
        }

        // Комментарии
        var methodComment, requestComment, responseComment string
        if val, ok := methodParameters["leading_comment"]; ok {
            methodComment = comment.ReplacePlaceholders(val.Val.String(), string(entityPrefDesc.Name()), linkedTypeName)
        }
        if val, ok := methodParameters["additional_leading_comment"]; ok {
            methodComment += ".\n" + val.Val.String()
        }
        if val, ok := methodParameters["request_leading_comment"]; ok {
            requestComment = comment.ReplacePlaceholders(val.Val.String(), string(entityPrefDesc.Name()), linkedTypeName)
        }
        if val, ok := methodParameters["response_leading_comment"]; ok {
            responseComment = comment.ReplacePlaceholders(val.Val.String(), string(entityPrefDesc.Name()), linkedTypeName)
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

    entityPrefDesc.Options().ProtoReflect().Clear(apiSpecOpt.Desc) // Удаляем опцию
    entityMessageProtodesc := protodesc.ToDescriptorProto(entityPrefDesc)

    // Описание сервисов
    apiSpecMap := util.GetFieldMap(apiSpecOpt.Val.Message())
    serviceSetVal, ok := apiSpecMap["service_set"]
    if !ok {
        return fmt.Errorf("не указан service_set в спецификации %s", entityPrefDesc.FullName())
    }

    serviceSetMap := util.GetFieldMap(serviceSetVal.Val.Message())

    for _, entitySourceService := range serviceSetMap {
        entityServiceMap := util.GetFieldMap(entitySourceService.Val.Message())

        // Описание сервиса из шаблона
        tmplServiceOptions := entitySourceService.Val.Message().Descriptor().Options().ProtoReflect()
        tmplServiceFieldMapVal := util.GetFieldMap(tmplServiceOptions)["service_tmpl"]
        if tmplServiceFieldMapVal.Val.String() == "<nil>" {
            return fmt.Errorf("не найден шаблон service_tmpl для сервиса в %s", entityPrefDesc.FullName())
        }
        tmplServiceMap := util.GetFieldMap(tmplServiceFieldMapVal.Val.Message())

        // Атрибуты сервиса
        serviceParameters := make(map[string]util.Field)
        maps.Copy(serviceParameters, tmplServiceMap)      // Из шаблона
        maps.Copy(serviceParameters, entityServiceMap)    // Из описания (переопределяют)

        serviceName := comment.ReplacePlaceholders(serviceParameters["name"].Val.String(), string(entityPrefDesc.Name()), "")
        serviceComment := comment.ReplacePlaceholders(serviceParameters["leading_comment"].Val.String(), string(entityPrefDesc.Name()), "")

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
        var requiredMethods map[string]util.Field
        if methodSetVal, ok := entityServiceMap["method_set"]; ok {
            requiredMethods = util.GetFieldMap(methodSetVal.Val.Message())
        }

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
                    method.Val.Message(),  // Вариация метода
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

        // Добавляем (обновленный) сервис обратно
        genFileProto.Service = append(genFileProto.Service, genServiceProto)
    }

    // Добавляем саму сущность
    genFileProto.MessageType = append(genFileProto.MessageType, entityMessageProtodesc)
    return nil
}