package comment

import (
	"entity-house/internal/util"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/builder"
)

// GetMessageComments собирает комментарии для сообщений и их полей, вложенных типов и т.д.
func GetMessageComments(messJhumpDesc *desc.MessageDescriptor, commentsMap map[string]string) {
	if messJhumpDesc.GetSourceInfo().LeadingComments != nil {
		commentsMap[messJhumpDesc.GetFullyQualifiedName()] = *messJhumpDesc.GetSourceInfo().LeadingComments
	}
	for _, nestedMsg := range messJhumpDesc.GetNestedMessageTypes() {
		GetMessageComments(nestedMsg, commentsMap)
	}
	for _, field := range messJhumpDesc.GetFields() {
		if field.GetSourceInfo().LeadingComments != nil {
			commentsMap[messJhumpDesc.GetFullyQualifiedName()+"."+field.GetName()] = *field.GetSourceInfo().LeadingComments
		}
	}
	for _, oneof := range messJhumpDesc.GetOneOfs() {
		if oneof.GetSourceInfo().LeadingComments != nil {
			commentsMap[messJhumpDesc.GetFullyQualifiedName()+"."+oneof.GetName()] = *oneof.GetSourceInfo().LeadingComments
		}
	}
	for _, enum := range messJhumpDesc.GetNestedEnumTypes() {
		if enum.GetSourceInfo().LeadingComments != nil {
			commentsMap[enum.GetFullyQualifiedName()] = *enum.GetSourceInfo().LeadingComments
		}
		for _, enumValue := range enum.GetValues() {
			if enumValue.GetSourceInfo().LeadingComments != nil {
				commentsMap[enumValue.GetFullyQualifiedName()] = *enumValue.GetSourceInfo().LeadingComments
			}
		}
	}
}

// FillMessageComments заполняет комментарии в builder'е сообщения.
func FillMessageComments(messJhumpDesc *desc.MessageDescriptor, messBuilder *builder.MessageBuilder, commentsMap map[string]string) {
	if val, ok := commentsMap[messJhumpDesc.GetFullyQualifiedName()]; ok {
		messBuilder.SetComments(builder.Comments{LeadingComment: val})
	}
	for _, nestedMsgDesc := range messJhumpDesc.GetNestedMessageTypes() {
		nestedMsgBuilder := messBuilder.GetNestedMessage(nestedMsgDesc.GetName())
		if nestedMsgBuilder != nil {
			FillMessageComments(nestedMsgDesc, nestedMsgBuilder, commentsMap)
		}
	}
	for _, fieldDesc := range messJhumpDesc.GetFields() {
		if val, ok := commentsMap[messJhumpDesc.GetFullyQualifiedName()+"."+fieldDesc.GetName()]; ok {
			fieldBuilder := messBuilder.GetField(fieldDesc.GetName())
			if fieldBuilder != nil {
				fieldBuilder.SetComments(builder.Comments{LeadingComment: val})
			}
		}
	}
	for _, oneofDesc := range messJhumpDesc.GetOneOfs() {
		if val, ok := commentsMap[messJhumpDesc.GetFullyQualifiedName()+"."+oneofDesc.GetName()]; ok {
			oneofBuilder := messBuilder.GetOneOf(oneofDesc.GetName())
			if oneofBuilder != nil {
				oneofBuilder.SetComments(builder.Comments{LeadingComment: val})
			}
		}
	}
	for _, enumDesc := range messJhumpDesc.GetNestedEnumTypes() {
		if val, ok := commentsMap[enumDesc.GetFullyQualifiedName()]; ok {
			enumBuilder := messBuilder.GetNestedEnum(enumDesc.GetName())
			if enumBuilder != nil {
				enumBuilder.SetComments(builder.Comments{LeadingComment: val})
			}
		}
		for _, enumValueDesc := range enumDesc.GetValues() {
			if val, ok := commentsMap[enumValueDesc.GetFullyQualifiedName()]; ok {
				enumBuilder := messBuilder.GetNestedEnum(enumDesc.GetName())
				if enumBuilder != nil {
					enumValueBuilder := enumBuilder.GetValue(enumValueDesc.GetName())
					if enumValueBuilder != nil {
						enumValueBuilder.SetComments(builder.Comments{LeadingComment: val})
					}
				}
			}
		}
	}
}

// ReplacePlaceholders заменяет стандартные плейсхолдеры в строке.
func ReplacePlaceholders(template, entityTypeName, entityKeyFieldDescription, entityTypeComment, sourceParameters string, methodParameters map[string]util.Field) string {
	s := strings.ReplaceAll(template, "{EntityTypeName}", entityTypeName)
	if entityTypeComment != "" {
		s = strings.ReplaceAll(s, "{EntityTypeComment}", entityTypeComment)
	}
	if entityKeyFieldDescription != "" {
		s = strings.ReplaceAll(s, "{EntityKeyFieldDescription}", strings.TrimSuffix(entityKeyFieldDescription, "\n"))
	}
	for k, v := range methodParameters {
		if v.Val.String() != "<nil>" {
			s = strings.ReplaceAll(s, "{"+sourceParameters+":"+k+"}", v.Val.String())
		}
	}
	for k, v := range methodParameters {
		if v.Val.String() != "<nil>" {
			s = strings.ReplaceAll(s, "{"+sourceParameters+":"+k+"}", v.Val.String())
		}
	}
	return s
}
