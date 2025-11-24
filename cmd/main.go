package main

import (
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strings"
    "entity-house/internal/generator"
)

func main() {
    protoPath := os.Getenv("PROTO_PATH")
    protoOutPath := os.Getenv("PROTO_OUT_PATH")
    protoImportPath := os.Getenv("PROTO_IMPORT_PATH")

    fmt.Println("PROTO_PATH", protoPath)
    fmt.Println("PROTO_OUT_PATH", protoOutPath)
    fmt.Println("PROTO_IMPORT_PATH", protoImportPath)

    if protoPath == "" {
        log.Fatal("PROTO_PATH environment variable is not set")
    }

    importPaths := strings.Split(protoImportPath, ";")
    // Убираем пустые пути, если есть
    var cleanImportPaths []string
    for _, path := range importPaths {
        if path != "" {
            cleanImportPaths = append(cleanImportPaths, path)
        }
    }

    generatedFiles, err := generator.BuildEntityFeatures(protoPath, cleanImportPaths)
    if err != nil {
        log.Fatalf("Failed to build entity features: %v", err)
    }

    fmt.Println("Generation end")

    if protoOutPath != "" {
        for filename, content := range generatedFiles {
            fullPath := filepath.Join(protoOutPath, filename)
            dir := filepath.Dir(fullPath)
            err := os.MkdirAll(dir, 0755)
            if err != nil {
                log.Printf("Failed to create directory %s: %v", dir, err)
                continue
            }
            err = os.WriteFile(fullPath, []byte(content), 0644)
            if err != nil {
                log.Printf("Failed to write to file %s: %v", fullPath, err)
            } else {
                fmt.Printf("Wrote file: %s\n", fullPath)
            }
        }
    }
}