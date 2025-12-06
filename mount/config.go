package mount

import (
	"encoding/json"
	"fmt"
	"muletinha/input"
	"os"
	"sync"
	"time"
)

type MountConfig struct {
	MountKey string `json:"mount_key"`
	SkillKey string `json:"skill_key"`
	Enabled  bool   `json:"enabled"`

	// Estado
	lastAddr     uint32
	mutex        sync.RWMutex
	lastMountKey time.Time
	lastSkillKey time.Time
	cooldown     time.Duration
}

func NewMountConfig() *MountConfig {
	mc := &MountConfig{
		MountKey: "LSHIFT+G",
		SkillKey: "LSHIFT+R",
		Enabled:  true,
		cooldown: 500 * time.Millisecond,
	}
	mc.LoadFromFile("mount_config.json")
	return mc
}

func (mc *MountConfig) LoadFromFile(filename string) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("[Mount] Config não encontrado, criando padrão\n")
		mc.SaveToFile(filename)
		return err
	}

	if err := json.Unmarshal(data, mc); err != nil {
		fmt.Printf("[Mount] Erro ao parsear JSON: %v\n", err)
		return err
	}

	fmt.Printf("[Mount] Config: mount=%s skill=%s enabled=%v\n", mc.MountKey, mc.SkillKey, mc.Enabled)
	return nil
}

func (mc *MountConfig) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(mc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// Update é chamado todo frame
func (mc *MountConfig) Update(addr uint32, name string) {
	if !mc.Enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	hasMount := addr != 0
	hadMount := mc.lastAddr != 0

	if hasMount {
		fmt.Printf("[Mount DEBUG] addr=0x%X name=%s\n", addr, name)
	}

	if hasMount && !hadMount {
		if mc.MountKey != "" && time.Since(mc.lastMountKey) >= mc.cooldown {
			fmt.Printf("[Mount] ★ %s detectada → %s\n", name, mc.MountKey)
			go input.SendKeyCombo(input.ParseKeyCombo(mc.MountKey))
			mc.lastMountKey = time.Now()
		}
	}

	if !hasMount && hadMount {
		fmt.Printf("[Mount] Desmontou\n")
	}

	mc.lastAddr = addr
}

func (mc *MountConfig) IsMounted() bool {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	return mc.lastAddr != 0
}