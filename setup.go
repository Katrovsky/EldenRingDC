package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

var stdinScanner = bufio.NewScanner(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	stdinScanner.Scan()
	return strings.TrimSpace(stdinScanner.Text())
}

// selectProfile показывает список персонажей и возвращает выбранный.
// Если персонаж один — выбирается автоматически без интерактива.
// Если несколько — навигация стрелками ↑/↓, выбор Enter.
func selectProfile(profiles []Profile) Profile {
	if len(profiles) == 1 {
		fmt.Printf("Character: %s (Level %d, Slot %d) — selected automatically\n",
			profiles[0].Name, profiles[0].Level, profiles[0].SlotIndex)
		return profiles[0]
	}

	return arrowSelect(profiles)
}

// arrowSelect — интерактивный выбор стрелками в raw-режиме терминала.
func arrowSelect(profiles []Profile) Profile {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// Терминал не поддерживает raw-режим — fallback на ввод номера
		return fallbackSelect(profiles)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	cursor := 0
	render := func() {
		// Переходим в начало блока: поднимаемся на len(profiles)+1 строк вверх
		fmt.Printf("\r\033[%dA", len(profiles)+1)
		fmt.Print("\r\033[KSelect character (↑/↓ arrows, Enter to confirm):\r\n")
		for i, p := range profiles {
			if i == cursor {
				fmt.Printf("\r\033[K  \033[7m▶  %s  (Level %d, Slot %d)\033[0m\r\n",
					p.Name, p.Level, p.SlotIndex)
			} else {
				fmt.Printf("\r\033[K     %s  (Level %d, Slot %d)\r\n",
					p.Name, p.Level, p.SlotIndex)
			}
		}
	}

	// Первичная отрисовка — просто печатаем, без подъёма
	fmt.Print("Select character (↑/↓ arrows, Enter to confirm):\r\n")
	for i, p := range profiles {
		if i == cursor {
			fmt.Printf("  \033[7m▶  %s  (Level %d, Slot %d)\033[0m\r\n",
				p.Name, p.Level, p.SlotIndex)
		} else {
			fmt.Printf("     %s  (Level %d, Slot %d)\r\n",
				p.Name, p.Level, p.SlotIndex)
		}
	}

	buf := make([]byte, 3)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			continue
		}

		switch {
		case n == 1 && buf[0] == 13: // Enter
			// Восстановить терминал до вывода следующего текста
			term.Restore(int(os.Stdin.Fd()), oldState)
			fmt.Printf("\r\nSelected: %s (Level %d, Slot %d)\r\n",
				profiles[cursor].Name, profiles[cursor].Level, profiles[cursor].SlotIndex)
			return profiles[cursor]

		case n == 1 && buf[0] == 3: // Ctrl+C
			term.Restore(int(os.Stdin.Fd()), oldState)
			fmt.Println("\r\nSetup cancelled.")
			os.Exit(0)

		case n == 3 && buf[0] == 27 && buf[1] == 91 && buf[2] == 65: // ↑
			if cursor > 0 {
				cursor--
				render()
			}

		case n == 3 && buf[0] == 27 && buf[1] == 91 && buf[2] == 66: // ↓
			if cursor < len(profiles)-1 {
				cursor++
				render()
			}
		}
	}
}

// fallbackSelect — числовой ввод, если raw-режим недоступен (CI, pipe и т.п.).
func fallbackSelect(profiles []Profile) Profile {
	fmt.Println("Found characters:")
	for _, p := range profiles {
		fmt.Printf("  Slot %d: %s (Level %d)\n", p.SlotIndex, p.Name, p.Level)
	}

	for {
		input := readLine("Enter slot number: ")
		var slot int
		if _, err := fmt.Sscan(input, &slot); err != nil {
			fmt.Println("Invalid input. Please enter a number.")
			continue
		}
		for _, p := range profiles {
			if p.SlotIndex == slot {
				return p
			}
		}
		fmt.Printf("Slot %d not found. Please choose from the list above.\n", slot)
	}
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

	fmt.Printf("\nFound save file at: %s\n\n", savePath)

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

	selected := selectProfile(profiles)

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
		CharacterSlot:  selected.SlotIndex,
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

	if err = os.WriteFile(configPath, data, 0644); err != nil {
		fmt.Printf("Error writing config: %v\n", err)
		return false
	}

	fmt.Printf("\nConfiguration saved to: %s\n", configPath)
	fmt.Println("Setup complete!")
	return true
}
