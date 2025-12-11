package monitor

import (
	"encoding/json"
	"fmt"
	"muletinha/config"
	"muletinha/input"
	"os"
	"time"
)

// ================== BUFF INFO ==================

type BuffInfo struct {
	Index    int
	ID       uint32
	Duration uint32
	TimeLeft uint32
	Name     string
}

type BuffEvent struct {
	Time    time.Time
	Type    string
	ID      uint32
	Name    string
	Reacted bool
}

// ================== DEBUFF INFO ==================

type DebuffInfo struct {
	Index   int
	ID      uint32
	TypeID  uint32
	DurMax  uint32
	DurLeft uint32
	CCName  string
}

type DebuffEvent struct {
	Time    time.Time
	Type    string
	ID      uint32
	TypeID  uint32
	CCName  string
	Reacted bool
}

func MakeKey(id, typeID uint32) uint64 {
	return (uint64(id) << 32) | uint64(typeID)
}

// ================== BUFF WHITELIST ==================

type BuffWhitelistEntry struct {
	Type     uint32         `json:"type"`
	Name     string         `json:"name"`
	Use      string         `json:"use"`
	KeyCombo input.KeyCombo `json:"-"`
}

type BuffWhitelist struct {
	Entries      []BuffWhitelistEntry
	TypeMap      map[uint32]*BuffWhitelistEntry
	Enabled      bool
	Reactions    int
	SpamCount    int
	SpamInterval time.Duration
	lastSpamTime time.Time
	spamCooldown time.Duration
}

func NewBuffWhitelist() *BuffWhitelist {
	wl := &BuffWhitelist{
		Entries:      make([]BuffWhitelistEntry, 0),
		TypeMap:      make(map[uint32]*BuffWhitelistEntry),
		Enabled:      true,
		SpamCount:    config.KEY_SPAM_COUNT,
		SpamInterval: config.KEY_SPAM_INTERVAL,
		spamCooldown: 100 * time.Millisecond,
	}
	wl.LoadFromFile("buff_whitelist.json")
	return wl
}

func (wl *BuffWhitelist) LoadFromFile(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		wl.createDefaultFile(filename)
		return
	}

	var entries []BuffWhitelistEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Printf("[BUFF] Erro JSON: %v\n", err)
		return
	}

	wl.Entries = entries
	wl.TypeMap = make(map[uint32]*BuffWhitelistEntry)

	for i := range wl.Entries {
		wl.Entries[i].KeyCombo = input.ParseKeyCombo(wl.Entries[i].Use)
		if wl.Entries[i].KeyCombo.MainKey != 0 {
			wl.TypeMap[wl.Entries[i].Type] = &wl.Entries[i]
		}
	}

	fmt.Printf("[BUFF] Carregado %d buffs da whitelist\n", len(wl.Entries))
}

func (wl *BuffWhitelist) createDefaultFile(filename string) {
	defaultEntries := []BuffWhitelistEntry{
		{Type: 87, Name: "Hell Spear", Use: "F10"},
		{Type: 243, Name: "stun", Use: "SHIFT+1"},
		{Type: 156, Name: "Fear", Use: "CTRL+2"},
		{Type: 21402, Name: "Deafened", Use: "ALT+F1"},
		{Type: 8000210, Name: "Clash Dummy", Use: "SHIFT+5"},
		{Type: 21, Name: "Tripped (Strong)", Use: "CTRL+SHIFT+1"},
		{Type: 141, Name: "Tripped", Use: "9"},
		{Type: 6860, Name: "Impaled", Use: "SHIFT+F10"},
		{Type: 18396, Name: "Skewer", Use: "F10"},
		{Type: 2458, Name: "Snare (charge)", Use: "F11"},
		{Type: 6829, Name: "Throw Dagger", Use: "CTRL+F11"},
		{Type: 501, Name: "Shield Slam", Use: "F10"},
		{Type: 3601, Name: "Overrun", Use: "SHIFT+F12"},
	}

	data, _ := json.MarshalIndent(defaultEntries, "", "  ")
	os.WriteFile(filename, data, 0644)
	fmt.Printf("[BUFF] Criado arquivo %s com %d entradas padrão\n", filename, len(defaultEntries))

	wl.Entries = defaultEntries
	wl.TypeMap = make(map[uint32]*BuffWhitelistEntry)
	for i := range wl.Entries {
		wl.Entries[i].KeyCombo = input.ParseKeyCombo(wl.Entries[i].Use)
		if wl.Entries[i].KeyCombo.MainKey != 0 {
			wl.TypeMap[wl.Entries[i].Type] = &wl.Entries[i]
		}
	}
}

func (wl *BuffWhitelist) ReactInstant(buffID uint32) (bool, string) {
	if !wl.Enabled {
		return false, ""
	}

	entry, exists := wl.TypeMap[buffID]
	if !exists {
		return false, ""
	}

	if time.Since(wl.lastSpamTime) < wl.spamCooldown {
		return false, ""
	}

	wl.lastSpamTime = time.Now()
	go input.SpamKey(entry.KeyCombo.RawString, wl.SpamCount, wl.SpamInterval)  // <- aqui

	wl.Reactions++
	return true, entry.Name
}

func (wl *BuffWhitelist) GetName(buffID uint32) string {
	if entry, exists := wl.TypeMap[buffID]; exists {
		return entry.Name
	}
	return ""
}

// ================== CC WHITELIST ==================

type CCWhitelistEntry struct {
	Type     uint32         `json:"type"`
	Name     string         `json:"name"`
	Use      string         `json:"use"`
	KeyCombo input.KeyCombo `json:"-"`
}

type CCWhitelist struct {
	Entries      []CCWhitelistEntry
	TypeMap      map[uint32]*CCWhitelistEntry
	Enabled      bool
	Reactions    int
	SpamCount    int
	SpamInterval time.Duration
	lastSpamTime time.Time
	spamCooldown time.Duration
}

func NewCCWhitelist() *CCWhitelist {
	wl := &CCWhitelist{
		Entries:      make([]CCWhitelistEntry, 0),
		TypeMap:      make(map[uint32]*CCWhitelistEntry),
		Enabled:      true,
		SpamCount:    config.KEY_SPAM_COUNT,
		SpamInterval: config.KEY_SPAM_INTERVAL,
		spamCooldown: 100 * time.Millisecond,
	}
	wl.LoadFromFile("cc_whitelist.json")
	return wl
}

func (wl *CCWhitelist) LoadFromFile(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		wl.createDefaultFile(filename)
		return
	}

	var entries []CCWhitelistEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Printf("[CC] Erro JSON: %v\n", err)
		return
	}

	wl.Entries = entries
	wl.TypeMap = make(map[uint32]*CCWhitelistEntry)

	for i := range wl.Entries {
		wl.Entries[i].KeyCombo = input.ParseKeyCombo(wl.Entries[i].Use)
		if wl.Entries[i].KeyCombo.MainKey != 0 {
			wl.TypeMap[wl.Entries[i].Type] = &wl.Entries[i]
		}
	}

	fmt.Printf("[CC] Carregado %d CCs\n", len(wl.Entries))
}

func (wl *CCWhitelist) createDefaultFile(filename string) {
	defaultEntries := []CCWhitelistEntry{
		{Type: 3601, Name: "stun", Use: "F12"},
		{Type: 509, Name: "knockdown", Use: "SHIFT+F12"},
		{Type: 4622, Name: "sleep", Use: "CTRL+F11"},
		{Type: 6800, Name: "fear", Use: "F12"},
		{Type: 20121, Name: "silence", Use: "SHIFT+1"},
		{Type: 22290, Name: "root", Use: "CTRL+2"},
	}

	data, _ := json.MarshalIndent(defaultEntries, "", "  ")
	os.WriteFile(filename, data, 0644)

	wl.Entries = defaultEntries
	wl.TypeMap = make(map[uint32]*CCWhitelistEntry)
	for i := range wl.Entries {
		wl.Entries[i].KeyCombo = input.ParseKeyCombo(wl.Entries[i].Use)
		if wl.Entries[i].KeyCombo.MainKey != 0 {
			wl.TypeMap[wl.Entries[i].Type] = &wl.Entries[i]
		}
	}
}

func (wl *CCWhitelist) ReactInstant(typeID uint32) (bool, string) {
	if !wl.Enabled {
		return false, ""
	}

	entry, exists := wl.TypeMap[typeID]
	if !exists {
		return false, ""
	}

	if time.Since(wl.lastSpamTime) < wl.spamCooldown {
		return false, ""
	}

	wl.lastSpamTime = time.Now()
	go input.SpamKey(entry.KeyCombo.RawString, wl.SpamCount, wl.SpamInterval)  // <- aqui

	wl.Reactions++
	return true, entry.Name
}

func (wl *CCWhitelist) GetName(typeID uint32) string {
	if entry, exists := wl.TypeMap[typeID]; exists {
		return entry.Name
	}
	return ""
}

// ================== BUFF MONITOR ==================

type BuffMonitor struct {
	Enabled      bool
	BuffListAddr uintptr
	Buffs        []BuffInfo
	KnownIDs     map[uint32]bool
	Events       []BuffEvent
	MaxEvents    int
	RawCount     uint32
	Whitelist    *BuffWhitelist
}

func NewBuffMonitor() *BuffMonitor {
	return &BuffMonitor{
		Enabled:   true,
		KnownIDs:  make(map[uint32]bool),
		Events:    make([]BuffEvent, 0, 20),
		MaxEvents: 20,
		Whitelist: NewBuffWhitelist(),
	}
}

func (m *BuffMonitor) AddEvent(eventType string, id uint32, name string, reacted bool) {
	event := BuffEvent{
		Time:    time.Now(),
		Type:    eventType,
		ID:      id,
		Name:    name,
		Reacted: reacted,
	}
	m.Events = append(m.Events, event)
	if len(m.Events) > m.MaxEvents {
		copy(m.Events, m.Events[1:])
		m.Events = m.Events[:m.MaxEvents]
	}
}

// ================== DEBUFF MONITOR ==================

type DebuffMonitor struct {
	Enabled     bool
	DebuffBase  uintptr
	Debuffs     []DebuffInfo
	KnownIDs    map[uint64]bool
	Events      []DebuffEvent
	MaxEvents   int
	RawCount    uint32
	CCWhitelist *CCWhitelist
}

func NewDebuffMonitor() *DebuffMonitor {
	return &DebuffMonitor{
		Enabled:     true,
		KnownIDs:    make(map[uint64]bool),
		Events:      make([]DebuffEvent, 0, 20),
		MaxEvents:   20,
		CCWhitelist: NewCCWhitelist(),
	}
}

func (m *DebuffMonitor) AddEvent(eventType string, id, typeID uint32, ccName string, reacted bool) {
	event := DebuffEvent{
		Time:    time.Now(),
		Type:    eventType,
		ID:      id,
		TypeID:  typeID,
		CCName:  ccName,
		Reacted: reacted,
	}
	m.Events = append(m.Events, event)
	if len(m.Events) > m.MaxEvents {
		copy(m.Events, m.Events[1:])
		m.Events = m.Events[:m.MaxEvents]
	}
}