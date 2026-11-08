package store

import (
	"os"
	"path/filepath"
	"strings"
)

var powerSupplyRoot = "/sys/class/power_supply"

func onBattery(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	hasBattery := false
	mainsOnline := false
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		typeBytes, err := os.ReadFile(filepath.Join(dir, "type"))
		if err != nil {
			continue
		}
		supplyType := strings.TrimSpace(string(typeBytes))
		if supplyType == "Battery" {
			hasBattery = true
			continue
		}
		if supplyType == "Mains" {
			online, err := os.ReadFile(filepath.Join(dir, "online"))
			if err == nil && strings.TrimSpace(string(online)) == "1" {
				mainsOnline = true
			}
		}
	}
	return hasBattery && !mainsOnline
}
