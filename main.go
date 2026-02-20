package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"unicode/utf16"
)

func init() {
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(30 << 20)
	runtime.GOMAXPROCS(1)
}

const (
	SlotStartIndex             = 0x310
	SlotLength                 = 0x280000
	SlotScanSize               = 0x40000
	NumSlots                   = 10
	SaveHeaderStartIndex       = 0x1901D0E
	SaveHeaderLength           = 0x24C
	CharActiveStatusStartIndex = 0x1901D04
	CharNameLength             = 0x22
	CharLevelOffset            = 0x22
	CharPlaytimeOffset         = 0x26
)

type Config struct {
	CharacterSlot  int    `json:"character_slot"`
	EnableWebUI    bool   `json:"enable_web_ui"`
	EnableTextFile bool   `json:"enable_text_file"`
	WebPort        int    `json:"web_port"`
	SavePath       string `json:"save_path,omitempty"`
}

type Profile struct {
	SlotIndex int
	Name      string
	Level     uint16
	PlayTime  uint32
	Deaths    int
	Active    bool
}

func main() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Error getting executable path: %v\n", err)
		return
	}
	exeDir := filepath.Dir(exePath)
	configPath := filepath.Join(exeDir, "config.json")

	_, err = loadConfig(configPath)
	if err != nil {
		if !runSetupWizard(configPath) {
			return
		}
	}

	runService(configPath, exeDir)
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func getSavePath(config *Config) (string, error) {
	if config != nil && config.SavePath != "" {
		if _, err := os.Stat(config.SavePath); err == nil {
			return config.SavePath, nil
		}
		return "", fmt.Errorf("save file not found at configured path: %s", config.SavePath)
	}

	baseDir := filepath.Join(os.Getenv("APPDATA"), "EldenRing")

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("EldenRing folder not found at %s", baseDir)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			fullPath := filepath.Join(baseDir, entry.Name(), "ER0000.sl2")
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath, nil
			}
		}
	}
	return "", fmt.Errorf("save file ER0000.sl2 not found")
}

func readSlotData(f *os.File, slotIndex int) (Profile, bool) {
	activeBuf := make([]byte, 1)
	if _, err := f.ReadAt(activeBuf, int64(CharActiveStatusStartIndex+slotIndex)); err != nil || activeBuf[0] != 1 {
		return Profile{}, false
	}

	headerOff := int64(SaveHeaderStartIndex + slotIndex*SaveHeaderLength)
	header := make([]byte, SaveHeaderLength)
	if _, err := f.ReadAt(header, headerOff); err != nil {
		return Profile{}, false
	}
	name := decodeUTF16(header[:CharNameLength])
	level := binary.LittleEndian.Uint16(header[CharLevelOffset : CharLevelOffset+2])
	playTime := binary.LittleEndian.Uint32(header[CharPlaytimeOffset : CharPlaytimeOffset+4])

	slotOff := int64(SlotStartIndex + slotIndex*SlotLength)
	scanBuf := make([]byte, SlotScanSize)
	if _, err := f.ReadAt(scanBuf, slotOff); err != nil {
		return Profile{}, false
	}
	deaths := findDeaths(scanBuf)

	return Profile{
		SlotIndex: slotIndex,
		Name:      name,
		Level:     level,
		PlayTime:  playTime,
		Deaths:    deaths,
		Active:    true,
	}, true
}

func parseSaveData(data []byte) []Profile {
	var profiles []Profile

	for i := range NumSlots {
		activeIdx := CharActiveStatusStartIndex + i
		if activeIdx >= len(data) || data[activeIdx] != 1 {
			continue
		}

		headerOffset := SaveHeaderStartIndex + (i * SaveHeaderLength)
		slotOffset := SlotStartIndex + (i * SlotLength)

		if headerOffset+SaveHeaderLength > len(data) {
			continue
		}

		nameBytes := data[headerOffset : headerOffset+CharNameLength]
		name := decodeUTF16(nameBytes)

		level := binary.LittleEndian.Uint16(data[headerOffset+CharLevelOffset : headerOffset+CharLevelOffset+2])
		playTime := binary.LittleEndian.Uint32(data[headerOffset+CharPlaytimeOffset : headerOffset+CharPlaytimeOffset+4])

		endSlot := min(slotOffset+SlotLength, len(data))
		deaths := findDeaths(data[slotOffset:endSlot])

		profiles = append(profiles, Profile{
			SlotIndex: i,
			Name:      name,
			Level:     level,
			PlayTime:  playTime,
			Deaths:    deaths,
			Active:    true,
		})
	}
	return profiles
}

func findDeaths(slotData []byte) int {
	if len(slotData) < 12 {
		return 0
	}

	for pos := 0; pos <= len(slotData)-12; pos++ {
		if slotData[pos+4] == 0xFF && slotData[pos+5] == 0xFF &&
			slotData[pos+6] == 0xFF && slotData[pos+7] == 0xFF &&
			slotData[pos+8] == 0x00 && slotData[pos+9] == 0x08 &&
			slotData[pos+10] == 0x00 && slotData[pos+11] == 0x00 {
			return int(binary.LittleEndian.Uint32(slotData[pos : pos+4]))
		}
	}
	return 0
}

func decodeUTF16(b []byte) string {
	u16s := make([]uint16, len(b)/2)
	for i := range u16s {
		u16s[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	runes := utf16.Decode(u16s)
	result := string(runes)
	if len(result) > 0 {
		for i := 0; i < len(result); i++ {
			if result[i] == 0 {
				return result[:i]
			}
		}
	}
	return result
}
