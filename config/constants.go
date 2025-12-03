package config

import "time"

// Memory offsets
const (
	PTR_LOCALPLAYER  = 0xE9DC54
	PTR_ENTITY       = 0x10
	OFF_ENTITY_BASE  = 0x38
	OFF_TO_ESI       = 0x4698
	OFF_TO_STATS     = 0x10
	OFF_MAXHP        = 0x420
	OFF_POS_X        = 0x830
	OFF_POS_Z        = 0x834
	OFF_POS_Y        = 0x838
	OFF_HP_ENTITY    = 0x84C
	OFF_NAME_PTR1    = 0x0C
	OFF_NAME_PTR2    = 0x1C

	// Debuff offsets
	OFF_DEBUFF_PTR   = 0x1898
	OFF_DEBUFF_COUNT = 0x20
	OFF_DEBUFF_ARRAY = 0xD30
	DEBUFF_SIZE      = 0x68

	// Buff offsets
	BUFF_COUNT_OFF = 0x20
	BUFF_ARRAY_OFF = 0x28
	BUFF_SIZE      = 0x68
	BUFF_OFF_SLOT  = 0x00
	BUFF_OFF_ID    = 0x04
	BUFF_OFF_DUR   = 0x30
	BUFF_OFF_LEFT  = 0x34
)

// Screen settings
const (
    SCREEN_WIDTH  = 1024
    SCREEN_HEIGHT = 768

    RADAR_RADIUS  = 280
    RADAR_RANGE   = 1000.0
    SCAN_RANGE    = 1000.0
)

// Key spam settings
const (
	KEY_SPAM_COUNT    = 5
	KEY_SPAM_INTERVAL = 15 * time.Millisecond
)