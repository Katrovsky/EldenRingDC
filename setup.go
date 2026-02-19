package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

var stdinScanner = bufio.NewScanner(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	stdinScanner.Scan()
	return strings.TrimSpace(stdinScanner.Text())
}

func runSetupWizard(configPath string) bool {
	fmt.Println("Elden Ring Death Counter - Setup Wizard")

	savePath, err := getSavePath(nil)
	if err != nil {
		fmt.Printf("Could not auto-detect save file: %v\n", err)
		savePath = readLine("Enter full path to ER0000.sl2: ")
		if _, err := os.Stat(savePath); err != nil {
			fmt.Printf("File not found: %s\n", savePath)
			return false
		}
	}

	fmt.Printf("\nFound save file at: %s\n", savePath)

	data, err := os.ReadFile(savePath)
	if err != nil {
		fmt.Printf("Error reading save file: %v\n", err)
		return false
	}

	profiles := parseSaveData(data)
	if len(profiles) == 0 {
		fmt.Println("No active characters found in save file.")
		return false
	}

	fmt.Println("\nFound characters:")
	for _, p := range profiles {
		fmt.Printf("Slot %d: %s (Level %d)\n", p.SlotIndex, p.Name, p.Level)
	}

	var slot int
	for {
		input := readLine("\nEnter character slot number: ")
		_, err := fmt.Sscan(input, &slot)
		if err != nil {
			fmt.Println("Invalid input. Please enter a number.")
			continue
		}

		if !slices.ContainsFunc(profiles, func(p Profile) bool { return p.SlotIndex == slot }) {
			fmt.Printf("Slot %d not found. Please choose from the list above.\n", slot)
			continue
		}
		break
	}

	enableWeb := readLine("\nEnable web overlay (Y/n): ")
	webUI := enableWeb != "n" && enableWeb != "N"

	var textFile bool
	if webUI {
		textFile = false
	} else {
		fmt.Println("Web overlay disabled. Text file output will be enabled.")
		textFile = true
	}

	port := 8080
	if webUI {
		portInput := readLine("Web server port (default 8080): ")
		if portInput != "" {
			p := 0
			fmt.Sscanf(portInput, "%d", &p)
			if p > 0 && p <= 65535 {
				port = p
			} else {
				fmt.Println("Invalid port, using 8080.")
			}
		}
	}

	config := Config{
		CharacterSlot:  slot,
		EnableWebUI:    webUI,
		EnableTextFile: textFile,
		WebPort:        port,
		SavePath:       savePath,
	}

	data, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("Error creating config: %v\n", err)
		return false
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		fmt.Printf("Error writing config: %v\n", err)
		return false
	}

	fmt.Printf("\nConfiguration saved to: %s\n", configPath)
	fmt.Printf("Save path: %s\n", savePath)
	fmt.Println("Setup complete!")
	return true
}
