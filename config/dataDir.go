package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultDataDirName = "data"

func GetDataDir() (string, error) {
	// Get the current executable's directory
	execPath, err := os.Executable()
	if err != nil {
		fmt.Println("Error getting executable path:", err)
		return "", err
	}
	execDir := filepath.Dir(execPath)

	// Create the full path for the new directory
	newDirPath := filepath.Join(execDir, defaultDataDirName)

	// Check if the directory exists
	if _, err := os.Stat(newDirPath); os.IsNotExist(err) {
		// Create the directory
		err := os.Mkdir(newDirPath, os.ModePerm)
		if err != nil {
			fmt.Println("Error creating directory:", err)
			return "", err
		}
		fmt.Println("Directory created:", newDirPath)
	}

	return newDirPath, nil
}
