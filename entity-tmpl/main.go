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
	pref "google.golang.org/protobuf/reflect/protoreflect"
	// "google.golang.org/protobuf/types/descriptorpb"
	// "github.com/bufbuild/protocompile/parser"
	// "google.golang.org/protobuf/proto"
	// "github.com/idsulik/go-collections/priorityqueue"
)

// обрабатываем methodSet сервиса
func methodSetHandler(field Field, entityName string, http_root string, service_unique_key string) {
	// в полях methodSet методы
	methodsMap := getFieldMap(field.val.Message())
	printFieldMap("methodsMap", methodsMap)
	for _, methodMessage := range methodsMap {
		method_unique_key := service_unique_key
		// в полях заданы параметры метода
		methodFieldMap := getFieldMap(methodMessage.val.Message())
		printFieldMap("methodFieldMap", methodFieldMap)
		if val, ok := methodFieldMap["unique_key"]; ok {
			method_unique_key = val.val.String()
		}
		fmt.Println("method_unique_key", method_unique_key)
		// в опциях параметры генерации
		methodOptsMap := getFieldMap(methodMessage.val.Message().Descriptor().Options().ProtoReflect())
		printFieldMap("methodOptsMap", methodOptsMap)
		// в полях entity.api.method_component_template_set шаблоны компонент метода
		methodTemplatesMap := getFieldMap(methodOptsMap["method_component_template_set"].val.Message())
		printFieldMap("methodTemplatesMap", methodTemplatesMap)
		// в полях entity.api.http_rule http параметры
		methodHttpRuleMap := getFieldMap(methodOptsMap["http_rule"].val.Message())
		printFieldMap("methodHttpRuleMap", methodHttpRuleMap)
		// entity.rules.key_field_behavior
		printEnumEl(methodOptsMap["key_field_behavior"])
	}
}

// Собираем map полей и значений protoreflect.Message
type Field struct {
	desc pref.FieldDescriptor
	val  pref.Value
}

func getFieldMap(message pref.Message) map[string]Field {
	var m map[string]Field = make(map[string]Field)
	message.Range(func(Desc pref.FieldDescriptor, Val pref.Value) bool {
		m[string(Desc.Name())] = Field{Desc, Val}
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
	}
}

func printEnumEl(el Field) {
	fmt.Println(el.desc.FullName(), " ", el.desc.Enum().FullName(), " ", el.val, " ", el.desc.Enum().Values().Get(int(el.val.Enum())).Name())
}

// обрабатываем заявленный ApiService
func apiServiceHandler(field Field, entityName string) {
	// в опции сервиса, есть только опция шаблона имени
	serviceOptsMap := getFieldMap(field.desc.Message().Options().ProtoReflect())
	printFieldMap("serviceOptsMap", serviceOptsMap)
	serviceName := strings.Replace(serviceOptsMap["name_template"].val.String(), "{EntityName}", entityName, -1)
	fmt.Println("serviceName", serviceName)
	// в полях сервиса требуемые методы в method_set и http_root
	serviceFieldsMap := getFieldMap(field.val.Message())
	printFieldMap("serviceFieldsMap", serviceFieldsMap)
	methodSetHandler(
		serviceFieldsMap["method_set"],
		entityName,
		serviceFieldsMap["http_root"].val.String(),
		serviceFieldsMap["unique_key"].val.String())
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

	if err != nil {
		log.Fatalf("Failed to parse proto files: %v", err)
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{
				".", "proto_deps",
			},
		}),
		SourceInfoMode: protocompile.SourceInfoMode(2),
		RetainASTs:     true,
	}

	entityFiles, err := compiler.Compile(context.Background(), "templates/device/v1/deviceapis_device_dtmf_v1.proto")
	if err != nil {
		fmt.Println("could not compile given files")
		return
	}

	// temlpateCompile, err := compiler.Compile(context.Background(), "entity-tmpl/entity-service-descriptor.proto")
	// if err != nil {
	// 	fmt.Println("could not compile given files")
	// 	return
	// }

	// temlpateDesc := temlpateCompile[len(temlpateCompile)-1]
	// fmt.Println(temlpateDesc)

	// var warningErrorsWithPos []reporter.ErrorWithPos
	// rep := reporter.NewReporter(
	// 	func(reporter.ErrorWithPos) error {
	// 		return nil
	// 	},
	// 	func(errorWithPos reporter.ErrorWithPos) {
	// 		warningErrorsWithPos = append(warningErrorsWithPos, errorWithPos)
	// 	},
	// )

	// reph := reporter.NewHandler(rep)

	type KeyEntry struct {
		Number int32
		Name   string
	}

	for _, entityFileDesc := range entityFiles {
		for i := range entityFileDesc.Messages().Len() {
			msgDesc := entityFileDesc.Messages().Get(i)
			//https://github.com/golang/protobuf/issues/1260
			// смотрим опции полей сущности
			// найдем выражение ключа сущности
			// var m map[string]*priorityqueue.PriorityQueue[KeyEntry] = make(map[string]*priorityqueue.PriorityQueue[KeyEntry])
			// pq := priorityqueue.New[KeyEntry](func(a, b KeyEntry) bool {
			// 	return a.Number < b.Number // Минимальный элемент имеет наивысший приоритет
			// })
			// for ii := range msgDesc.Fields().Len() {
			// 	fmt.Println("msgDesc.Fields().", msgDesc.Fields().Get(ii).Name())
			// 	msgDesc.Fields().Get(ii).Options().ProtoReflect().Range(func(fieldDesc pref.FieldDescriptor, fieldVal pref.Value) bool {
			// 		if fieldDesc.FullName() == "entity.feature.unique_key" {
			// 			fmt.Println("fieldDesc ", fieldDesc.FullName(), fieldVal.String())
			// 			pq.Push(KeyEntry{Number:int32(msgDesc.Fields().Get(ii).Number()), Name: string(msgDesc.Fields().Get(ii).Name())})
			// 			m[fieldVal.String()] = pq
			// 		}
			// 		return true
			// 	})
			// }
			// fmt.Println("unique_key", m)
			fmt.Println("efMessageHandler", msgDesc.Name())
			// ищем опции entity.feature
			msgOptsMap := getFieldMap(msgDesc.Options().ProtoReflect())
			printFieldMap("msgOptsMap", msgOptsMap)
			// смотрим какие именно заявлены фичи сущности
			// если entity.feature.api_service добавляем сервис, он описан в полях фичи, и он один
			apiServiceFieldMap := getFieldMap(msgOptsMap["api_service"].val.Message())
			printFieldMap("apiServiceFieldMap", apiServiceFieldMap)
			for _, val := range apiServiceFieldMap {
				apiServiceHandler(val, string(msgDesc.Name()))
			}
		}
	}
	// m := temlpateDesc.Messages().ByName("MethodCollection")
	// fmt.Println(m)
	fmt.Println("END")
}
