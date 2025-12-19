package config

import "time"

// Memory offsets
const (
	// ============ LOCAL PLAYER ============
	PTR_LOCALPLAYER  = 0xE9DC54
	PTR_ENTITY       = 0x10
	OFF_ENTITY_BASE  = 0x38

	PTR_MOUNT_BASE   uintptr = 0x000930BC
	OFF_MOUNT_PTR1   uint32  = 0x3C
	OFF_MOUNT_PTR2   uint32  = 0x4

	OFF_TO_ESI       = 0x4698
	OFF_TO_STATS     = 0x10
	OFF_MAXHP        = 0x420
	OFF_POS_X        = 0x830
	OFF_POS_Z        = 0x834
	OFF_POS_Y        = 0x838
	OFF_HP_ENTITY    = 0x84C
	OFF_NAME_PTR1    = 0x0C
	OFF_NAME_PTR2    = 0x1C


	// ============ LOCAL PLAYER MANA ============
	PTR_MANA_BASE     = 0x130D824
	OFF_MANA_PTR1     = 0x4
	OFF_MANA_PTR2     = 0x18
	OFF_MANA_PTR3     = 0xB0
	OFF_MANA_PTR4     = 0x10
	OFF_MANA_PTR5     = 0x5C
	OFF_MANA_PTR6     = 0x0
	OFF_MANA_CURRENT  = 0x318
	OFF_MANA_MAX      = 0x314

	// ============ TARGET (UI Structure) ============
	// Acessado via ESI na instrução x2game.dll+1F2BA
	// Nota: Ponteiro base é dinâmico, precisa ser obtido via breakpoint/hook
	OFF_TARGET_MAXHP   = 0x314
	OFF_TARGET_HP      = 0x318
	OFF_TARGET_MAXMANA = 0xD4C
	OFF_TARGET_MANA    = 0xD50
	OFF_TARGET_ID      = 0x008  // Entity ID (possível)
	OFF_TARGET_TYPE    = 0x020  // Entity Type (possível)
	OFF_TARGET_LEVEL   = 0x024  // Level (possível)

	// ============ BUFFS ============
	OFF_DEBUFF_PTR = 0x1898

	BUFF_COUNT_OFF = 0x20
	BUFF_ARRAY_OFF = 0x28
	BUFF_SIZE      = 0x68
	BUFF_OFF_SLOT  = 0x00
	BUFF_OFF_ID    = 0x04
	BUFF_OFF_DUR   = 0x30
	BUFF_OFF_LEFT  = 0x34

	OFF_DEBUFF_COUNT = 0xD28
	OFF_DEBUFF_ARRAY = 0xD30
	DEBUFF_SIZE      = 0x68

	PTR_BUFF_FREEZE      uintptr = 0x01325640
	OFF_BUFF_FREEZE_PTR1 uint32  = 0x4
	OFF_BUFF_FREEZE_PTR2 uint32  = 0x20
	OFF_BUFF_FREEZE_PTR3 uint32  = 0x8
	OFF_BUFF_FREEZE_FINAL uint32 = 0x384
)

// Screen settings
const (
    SCREEN_WIDTH  = 1920
    SCREEN_HEIGHT = 1080

    RADAR_RADIUS  = 280
    RADAR_RANGE   = 1000.0
    SCAN_RANGE    = 1000.0
)

// Key spam settings
const (
	KEY_SPAM_COUNT    = 5
	KEY_SPAM_INTERVAL = 15 * time.Millisecond
)