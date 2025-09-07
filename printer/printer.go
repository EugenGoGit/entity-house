package printer

import (
    "sort"
    "strings"

    // "github.com/jhump/protoreflect/desc/builder"
    "github.com/jhump/protoreflect/desc/protoprint"
)


// SortFunction определяет порядок сортировки элементов при печати.
func SortFunction(a, b protoprint.Element) bool {
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

// CreatePrinter создает и настраивает принтер.
func CreatePrinter() *protoprint.Printer {
    p := &protoprint.Printer{}
    p.CustomSortFunction = SortFunction
    p.SortElements = true
    p.Indent = "    "     // 4 пробела
    p.Compact = true 
    p.ForceFullyQualifiedNames = false
    return p
}

// AddLeadingComments добавляет комментарии в начало файла.
func AddLeadingComments(protoStr string, leadingComments []string) string {
    if len(leadingComments) == 0 {
        return protoStr
    }
    sort.Strings(leadingComments) // Сортируем для консистентности
    var commentBlock strings.Builder
    for _, comment := range leadingComments {
        commentBlock.WriteString("/*\n")
        commentBlock.WriteString(comment)
        commentBlock.WriteString("\n*/\n")
    }
    return commentBlock.String() + protoStr
}