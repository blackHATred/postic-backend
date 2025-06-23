package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Проверяем существование папки api
	apiProtoRoot := "api"
	if _, err := os.Stat(apiProtoRoot); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Directory %s does not exist\n", apiProtoRoot)
		os.Exit(1)
	}

	// Обрабатываем proto файлы в api/ директории
	err := filepath.Walk(apiProtoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".proto" {
			fmt.Printf("Generating gRPC for: %s\n", path)

			// Извлекаем имя сервиса из названия файла (без расширения)
			serviceName := strings.TrimSuffix(info.Name(), ".proto")

			// Создаем выходную директорию для сервиса
			outDir := fmt.Sprintf("internal/delivery/grpc/%s/proto", serviceName)
			if err := os.MkdirAll(outDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory %s: %v", outDir, err)
			}

			cmd := exec.Command(
				"protoc",
				fmt.Sprintf("--proto_path=%s", apiProtoRoot),
				fmt.Sprintf("--go_out=%s", outDir),
				fmt.Sprintf("--go-grpc_out=%s", outDir),
				"--go_opt=paths=source_relative",
				"--go-grpc_opt=paths=source_relative",
				info.Name(),
			)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to generate gRPC for %s: %v\n", path, err)
				return err
			}
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing api: %v\n", err)
	}

	fmt.Println("Proto generation complete.")
}
