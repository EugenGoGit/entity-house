package main

import (
	"bytes"
	"entity-house/generator"
	"strings"

	"fmt"
	"io"
	"log"
	"os"
	"testing"
)

func compareFilesByteByByte(fileOut, fileAssert string) (bool, error) {
	f1, err := os.Open(fileOut)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(fileAssert)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	buf1 := make([]byte, 128)
	buf2 := make([]byte, 128)

	for {
		n1, err1 := f1.Read(buf1)
		n2, err2 := f2.Read(buf2)

		if err1 != nil && err1 != io.EOF {
			return false, err1
		}
		if err2 != nil && err2 != io.EOF {
			return false, err2
		}

		if n1 != n2 || !bytes.Equal(buf1[:n1], buf2[:n2]) {
			fmt.Println("Files differ fileOut *************")
			fmt.Println(string(buf1[:n1]))
			fmt.Println("Files differ fileAssert *************")
			fmt.Println(string(buf2[:n2]))
			return false, nil // Files differ
		}

		if err1 == io.EOF && err2 == io.EOF {
			return true, nil // Files are identical
		}
		if err1 == io.EOF || err2 == io.EOF {
			fmt.Println("One file ended before the other")
			return false, nil // One file ended before the other
		}
	}
}

func TestGenProto(t *testing.T) {
	m, err := generator.BuildEntityFeatures("./test_proto", []string{".", "proto_deps", "./impl", "entity_feature"})
	if err != nil {
		panic(err)
	}
	fmt.Println("BuildEntityFeatures основная функция генерации.")
	bb := ""
	for genFileName, v := range m {
		// The file permissions (e.g., 0644 for read/write by owner, read-only by others)
		// You can adjust these permissions as needed.
		// permissions := os.FileMode(0644)

		assertFileName := strings.Replace(genFileName, "test_proto/", "test_assert/", 1)
		outFileName := assertFileName + "_out"
		err := os.WriteFile(outFileName, []byte(v), 0644)
		if err != nil {
			log.Fatalf("Failed to write to file: %v", err)
		}

		b, err := compareFilesByteByByte(outFileName, assertFileName)
		if err != nil {
			log.Fatalf("unable to read file: %v", err)
		}
		fmt.Println("assert: ", assertFileName, b)
		if !b {
			bb = bb + ";\n" + assertFileName
		}
	}
	if bb != "" {
		log.Fatalf("Failed assertion %v", bb)
	}
}
