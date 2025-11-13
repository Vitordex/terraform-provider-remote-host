package filesystem

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

type FileNotFoundError struct {
	error
}

func ReadFile(path string) ([]byte, error) {
	if !FileExists(path) {
		fmt.Printf("File %s not found\n", path)
		return nil, FileNotFoundError{error: errors.New("FileNotFoundError")}
	}

	file, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return file, nil
}

func DeleteFile(path string) error {
	if !FileExists(path) {
		fmt.Printf("File %s not found\n", path)
		return nil
	}

	return os.Remove(path)
}

func GetWorkingDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println(err.Error())
		panic(err)
	}

	return dir
}

func CreatePath(path string) {
	_, err := os.Stat(path)
	if err == nil {
		return
	}

	if !os.IsNotExist(err) {
		fmt.Printf("Error accessing path %s: %s\n", path, err.Error())
		panic(err)
	}

	pathComponents := strings.Split(path, string(os.PathSeparator))
	fmt.Printf("Path %s components: %v\n", path, pathComponents)

	for i, component := range pathComponents {
		currentPath := strings.Join(pathComponents[:i+1], string(os.PathSeparator))

		if currentPath == "" {
			currentPath = "/"
		}

		if FileExists(currentPath) {
			fmt.Printf("Path %s already exists\n", currentPath)
			continue
		}
		var err error
		var file *os.File
		isFile := strings.Contains(component, ".")

		if isFile {
			file, err = os.Create(currentPath)
		} else {
			err = os.Mkdir(currentPath, os.ModePerm)
		}

		if err != nil {
			fmt.Printf("Failed to create path %s: %s\n", currentPath, err.Error())
			panic(err)
		}

		if isFile {
			file.Close()
		}

		fmt.Printf("Path %s created successfully\n", currentPath)
	}
}

func ListDirectory(path string) *[]os.DirEntry {
	files, err := os.ReadDir(path)
	if err != nil {
		fmt.Printf("Failed to list directory %s: %s\n", path, err.Error())
		panic(err)
	}
	return &files
}
