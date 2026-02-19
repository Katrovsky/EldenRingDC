package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func runService(configPath, exeDir string) {
	config, err := loadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	var deathFilePath string
	if config.EnableTextFile {
		deathFilePath = filepath.Join(exeDir, "death.txt")
	}

	if config.EnableWebUI {
		go startWebServer(config.WebPort)
	}

	fmt.Printf("Monitoring character in slot %d\n", config.CharacterSlot)
	if config.EnableTextFile {
		fmt.Printf("Death count will be written to: %s\n", deathFilePath)
	}
	if config.EnableWebUI {
		fmt.Printf("Web overlay: http://localhost:%d\n", config.WebPort)
	}
	fmt.Println("Press Ctrl+C to stop")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var savePath string
	var lastModTime time.Time
	var lastDeaths int = -1

	for range ticker.C {
		if savePath == "" {
			p, err := getSavePath(config)
			if err != nil {
				continue
			}
			savePath = p
			fmt.Printf("Save file: %s\n", savePath)
		}

		info, err := os.Stat(savePath)
		if err != nil {
			savePath = ""
			lastModTime = time.Time{}
			continue
		}

		modTime := info.ModTime()
		if !modTime.After(lastModTime) {
			continue
		}

		f, err := os.Open(savePath)
		if err != nil {
			continue
		}

		profile, ok := readSlotData(f, config.CharacterSlot)
		if !ok {
			f.Close()
			time.Sleep(50 * time.Millisecond)
			f, err = os.Open(savePath)
			if err != nil {
				continue
			}
			profile, ok = readSlotData(f, config.CharacterSlot)
		}
		f.Close()

		if !ok {
			continue
		}

		lastModTime = modTime

		if profile.Deaths == lastDeaths {
			continue
		}
		lastDeaths = profile.Deaths
		if profile.Deaths == 0 {
			continue
		}

		if config.EnableWebUI {
			counter.Update(profile.Deaths, profile.Name)
		}
		if config.EnableTextFile {
			os.WriteFile(deathFilePath, []byte(fmt.Sprintf("%d", profile.Deaths)), 0644)
		}

		fmt.Printf("[%s] %s - Deaths: %d\n",
			time.Now().Format("15:04:05"),
			profile.Name,
			profile.Deaths)
	}
}
