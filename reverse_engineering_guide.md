# 🔍 Guia de Engenharia Reversa - ArcheAge Offsets

Este guia explica detalhadamente como os offsets de memória foram descobertos para ler dados do jogo ArcheAge.

## 📚 Índice

1. [Ferramentas Necessárias](#-ferramentas-necessárias)
2. [Conceitos Básicos](#-conceitos-básicos)
3. [Encontrando o LocalPlayer](#-encontrando-o-localplayer)
4. [Estrutura de Entidades](#-estrutura-de-entidades)
5. [Offsets de Posição](#-offsets-de-posição)
6. [Sistema de HP](#-sistema-de-hp)
7. [Sistema de Buffs/Debuffs](#-sistema-de-buffsdebuffs)
8. [Dicas e Truques](#-dicas-e-truques)

---

## 🛠️ Ferramentas Necessárias

| Ferramenta | Uso | Download |
|------------|-----|----------|
| **Cheat Engine** | Scanner de memória, debugger | [cheatengine.org](https://cheatengine.org) |
| **x64dbg/x32dbg** | Debugger assembly | [x64dbg.com](https://x64dbg.com) |
| **ReClass.NET** | Visualização de estruturas | [github.com/ReClassNET](https://github.com/ReClassNET/ReClass.NET) |
| **IDA Pro/Ghidra** | Disassembler estático | [hex-rays.com](https://hex-rays.com) / [ghidra-sre.org](https://ghidra-sre.org) |
| **Process Hacker** | Visualização de processos | [processhacker.sourceforge.io](https://processhacker.sourceforge.io) |

---

## 📖 Conceitos Básicos

### Módulos e Base Address

Quando um jogo carrega, cada DLL tem um **endereço base** diferente a cada execução (devido ao ASLR - Address Space Layout Randomization).
archeage.exe -> Base: 0x00400000 (geralmente fixo) x2game.dll -> Base: 0x39000000 (varia) CryGame.dll -> Base: 0x3A000000 (varia)


Por isso, usamos **offsets relativos** ao módulo:
Endereço Real = Base do Módulo + Offset


### Ponteiros e Chains

Dados importantes geralmente estão atrás de **cadeias de ponteiros**:
[[[x2game.dll + 0xE9DC54] + 0x10] + 0x38] + 0x84C = HP do Player


Isso significa:
1. Leia o valor em `x2game.dll + 0xE9DC54` → obtém Ponteiro1
2. Leia o valor em `Ponteiro1 + 0x10` → obtém Ponteiro2  
3. Leia o valor em `Ponteiro2 + 0x38` → obtém EntityBase
4. Leia o valor em `EntityBase + 0x84C` → obtém HP

---

## 🎯 Encontrando o LocalPlayer

### Passo 1: Encontrar o HP atual

1. Abra o **Cheat Engine** e conecte ao processo `archeage.exe`
2. Olhe seu HP no jogo (ex: 15847)
3. Faça um **First Scan** por esse valor (4 bytes, exact value)
4. Tome dano no jogo para o HP mudar (ex: 14523)
5. Faça **Next Scan** com o novo valor
6. Repita até restar poucos endereços (1-5)
Scan 1: HP = 15847 → 847293 resultados Scan 2: HP = 14523 → 12 resultados Scan 3: HP = 14102 → 3 resultados Scan 4: HP = 13856 → 1 resultado ✓


### Passo 2: Fazer Pointer Scan

1. Com o endereço do HP encontrado (ex: `0x1A5F384C`)
2. Clique direito → **Find out what accesses this address**
3. Tome dano novamente para gerar acessos
4. Você verá instruções assembly como:

```asm
mov eax, [esi+84C]      ; esi = EntityBase, 84C = offset do HP
cmp [ebx+84C], ecx      ; comparação de HP
O 84C é nosso offset do HP dentro da entidade!

Passo 3: Encontrar o EntityBase
No Cheat Engine, clique direito no endereço do HP
Selecione Pointer scan for this address
Configure:
Max level: 5
Max offset: 5000
Salve e analise os resultados
Resultado típico:

x2game.dll + E9DC54 → +10 → +38 → HP está em +84C
Passo 4: Validar a Chain
Teste manual no Cheat Engine:

1. [x2game.dll + E9DC54] = 0x3B8A1000 (ptr1)
2. [0x3B8A1000 + 10]     = 0x1A5F3000 (ptr2 - Entity Address)
3. [0x1A5F3000 + 38]     = 0x1A5F3800 (EntityBase)
4. [0x1A5F3800 + 84C]    = 15847     (HP!) ✓
Código Resultante
const (
    PTR_LOCALPLAYER = 0xE9DC54  // Offset na x2game.dll
    PTR_ENTITY      = 0x10      // Offset para Entity Address
    OFF_ENTITY_BASE = 0x38      // Offset para EntityBase
    OFF_HP_ENTITY   = 0x84C     // Offset para HP
)

func getLocalPlayerHP(handle windows.Handle, x2game uintptr) uint32 {
    // Passo 1: Ler primeiro ponteiro
    ptr1 := readU32(handle, x2game + PTR_LOCALPLAYER)
    // ptr1 agora contém algo como 0x3B8A1000
    
    // Passo 2: Ler endereço da entidade
    entityAddr := readU32(handle, uintptr(ptr1) + PTR_ENTITY)
    // entityAddr agora contém algo como 0x1A5F3000
    
    // Passo 3: Ler HP diretamente (ou via EntityBase)
    hp := readU32(handle, uintptr(entityAddr) + OFF_HP_ENTITY)
    // hp agora contém 15847
    
    return hp
}
Diagrama Visual
x2game.dll
    │
    ├──[+0xE9DC54]──► Ponteiro para estrutura do LocalPlayer
    │                      │
    │                      ├──[+0x10]──► Entity Address (0x1A5F3000)
    │                      │                  │
    │                      │                  ├──[+0x00]──► VTable
    │                      │                  ├──[+0x0C]──► Name Pointer 1
    │                      │                  ├──[+0x38]──► EntityBase
    │                      │                  │                 │
    │                      │                  │                 ├──[+0x1898]──► Debuff Pointer
    │                      │                  │                 └──[+0x4698]──► Stats Pointer
    │                      │                  │
    │                      │                  ├──[+0x830]──► Position X
    │                      │                  ├──[+0x834]──► Position Z
    │                      │                  ├──[+0x838]──► Position Y
    │                      │                  └──[+0x84C]──► Current HP
🏗️ Estrutura de Entidades
Usando ReClass.NET
Abra ReClass.NET e conecte ao processo
Crie uma nova classe no endereço da entidade
Mapeie os campos conhecidos:
class Entity {
    /* 0x000 */ void* vTable;           // Ponteiro para tabela virtual
    /* 0x004 */ uint32_t unknown1;
    /* 0x008 */ uint32_t unknown2;
    /* 0x00C */ char* namePtr1;         // Primeiro ponteiro do nome
    /* 0x010 */ uint32_t unknown3;
    // ... campos desconhecidos ...
    /* 0x038 */ EntityBase* base;       // Ponteiro para dados estendidos
    // ... mais campos ...
    /* 0x830 */ float posX;             // Posição X
    /* 0x834 */ float posZ;             // Posição Z (altura)
    /* 0x838 */ float posY;             // Posição Y
    // ... mais campos ...
    /* 0x84C */ uint32_t currentHP;     // HP atual
};
Identificando o VTable
O VTable (Virtual Table) é crucial para identificar o tipo de entidade:

// VTables típicos (variam por versão)
const (
    VTABLE_PLAYER = 0x39XXXXXX  // Players têm este VTable
    VTABLE_NPC    = 0x39YYYYYY  // NPCs têm este VTable
    VTABLE_MOB    = 0x39ZZZZZZ  // Mobs têm este VTable
)

func isPlayer(vtable uint32) bool {
    // Players geralmente têm VTable em uma faixa específica
    return vtable >= 0x39000000 && vtable < 0x3B000000
}
📍 Offsets de Posição
Método: Movimentação
Encontre sua posição X atual no mapa do jogo
Faça scan por float com valor aproximado
Mova-se apenas no eixo X
Faça Next Scan → Changed Value
Repita até isolar o endereço
Posição inicial: X=1000.5, Y=500.3
Scan: float, 1000.5 (com margem de 0.1)

Move para X=1050.5
Next Scan: float, 1050.5

Encontrado: 0x1A5F3830 = Position X
Descoberta dos Offsets Y e Z
Posições geralmente estão consecutivas na memória:

0x1A5F3830 = X (float)
0x1A5F3834 = Z (float) - altura
0x1A5F3838 = Y (float)
Calcule o offset:

Entity Address = 0x1A5F3000
Position X     = 0x1A5F3830

Offset = 0x1A5F3830 - 0x1A5F3000 = 0x830
Código
const (
    OFF_POS_X = 0x830
    OFF_POS_Z = 0x834  // Altura
    OFF_POS_Y = 0x838
)

func getPosition(handle windows.Handle, entityAddr uint32) (x, y, z float32) {
    x = readF32(handle, uintptr(entityAddr + OFF_POS_X))
    z = readF32(handle, uintptr(entityAddr + OFF_POS_Z))
    y = readF32(handle, uintptr(entityAddr + OFF_POS_Y))
    return
}
❤️ Sistema de HP
HP Atual vs HP Máximo
O HP atual está diretamente na entidade, mas o HP máximo está em uma estrutura separada de Stats:

Entity
  └──[+0x38]──► EntityBase
                  └──[+0x4698]──► ESI (Stats Pointer 1)
                                    └──[+0x10]──► Stats Structure
                                                    └──[+0x420]──► Max HP
Encontrando Max HP
Encontre seu Max HP no jogo (ex: 25000)
Scan por 4 bytes, valor exato
Equipe/desequipe item que muda Max HP
Next Scan com novo valor
Use Find what accesses para ver a chain
Assembly típico:

mov eax, [esi+10]       ; esi+10 = stats structure
mov ecx, [eax+420]      ; eax+420 = max HP
Código
const (
    OFF_ENTITY_BASE = 0x38
    OFF_TO_ESI      = 0x4698
    OFF_TO_STATS    = 0x10
    OFF_MAXHP       = 0x420
)

func getMaxHP(handle windows.Handle, entityAddr uint32) uint32 {
    // EntityBase
    base := readU32(handle, uintptr(entityAddr + OFF_ENTITY_BASE))
    if !isValidPtr(base) {
        return 0
    }
    
    // Stats Pointer 1
    esi := readU32(handle, uintptr(base + OFF_TO_ESI))
    if !isValidPtr(esi) {
        return 0
    }
    
    // Stats Structure
    stats := readU32(handle, uintptr(esi + OFF_TO_STATS))
    if !isValidPtr(stats) {
        return 0
    }
    
    // Max HP
    return readU32(handle, uintptr(stats + OFF_MAXHP))
}
🎭 Sistema de Buffs/Debuffs
Estrutura de Debuffs
Debuffs são armazenados em um array dentro de uma estrutura:

EntityBase
  └──[+0x1898]──► DebuffManager
                    ├──[+0x20]──► Count (quantidade de debuffs)
                    └──[+0xD30]──► Array de Debuffs
                                    ├── Debuff[0] (0x68 bytes cada)
                                    ├── Debuff[1]
                                    └── ...
Estrutura de um Debuff
struct Debuff {
    /* 0x00 */ uint32_t id;           // ID único do debuff
    /* 0x04 */ uint32_t typeID;       // Tipo/Skill ID
    /* 0x08 */ uint32_t unknown[10];  // Dados variados
    /* 0x30 */ uint32_t durationMax;  // Duração máxima (ms)
    /* 0x34 */ uint32_t durationLeft; // Tempo restante (ms)
    // ... até 0x68 bytes total
};
Encontrando a Estrutura de Debuffs
Aplique um debuff em si mesmo (ou peça para alguém aplicar)
Scan pelo ID do debuff ou duração
Use Find what accesses quando o debuff expira
Analise o código assembly:
mov ecx, [ebx+1898h]    ; ebx = EntityBase, 1898 = DebuffManager
mov eax, [ecx+20h]      ; count
lea esi, [ecx+D30h]     ; array start
Código
const (
    OFF_DEBUFF_PTR   = 0x1898
    OFF_DEBUFF_COUNT = 0x20
    OFF_DEBUFF_ARRAY = 0xD30
    DEBUFF_SIZE      = 0x68
)

type DebuffInfo struct {
    ID       uint32
    TypeID   uint32
    DurMax   uint32
    DurLeft  uint32
}

func getDebuffs(handle windows.Handle, entityBase uint32) []DebuffInfo {
    // Ler ponteiro do DebuffManager
    debuffMgr := readU32(handle, uintptr(entityBase + OFF_DEBUFF_PTR))
    if !isValidPtr(debuffMgr) {
        return nil
    }
    
    // Ler quantidade
    count := readU32(handle, uintptr(debuffMgr + OFF_DEBUFF_COUNT))
    if count == 0 || count > 50 {
        return nil
    }
    
    // Endereço do array
    arrayAddr := debuffMgr + OFF_DEBUFF_ARRAY
    
    // Ler todos os debuffs de uma vez (otimização)
    buffer := make([]byte, count * DEBUFF_SIZE)
    readMemoryBytes(handle, uintptr(arrayAddr), buffer)
    
    var debuffs []DebuffInfo
    for i := uint32(0); i < count; i++ {
        offset := i * DEBUFF_SIZE
        
        debuff := DebuffInfo{
            ID:      bytesToUint32(buffer[offset : offset+4]),
            TypeID:  bytesToUint32(buffer[offset+4 : offset+8]),
            DurMax:  bytesToUint32(buffer[offset+0x30 : offset+0x34]),
            DurLeft: bytesToUint32(buffer[offset+0x34 : offset+0x38]),
        }
        
        // Validação básica
        if debuff.ID > 0 && debuff.DurMax > 0 {
            debuffs = append(debuffs, debuff)
        }
    }
    
    return debuffs
}
Sistema de Buffs
Buffs têm estrutura similar mas em local diferente:

const (
    BUFF_COUNT_OFF = 0x20
    BUFF_ARRAY_OFF = 0x28
    BUFF_SIZE      = 0x68
    BUFF_OFF_ID    = 0x04
    BUFF_OFF_DUR   = 0x30
    BUFF_OFF_LEFT  = 0x34
)
A diferença principal é que o BuffManager precisa ser encontrado dinamicamente escaneando a região de memória do EntityBase.

💡 Dicas e Truques
1. Validação de Ponteiros
Sempre valide ponteiros antes de usar:

func isValidPtr(ptr uint32) bool {
    // Ponteiros válidos geralmente estão nesta faixa
    return ptr >= 0x10000000 && ptr < 0xF0000000
}
2. Leitura em Batch
Ler memória é lento. Leia blocos grandes de uma vez:

// RUIM - muitas chamadas
for i := 0; i < 100; i++ {
    value := readU32(handle, baseAddr + uintptr(i*4))
}

// BOM - uma chamada só
buffer := make([]byte, 400)
readMemoryBytes(handle, baseAddr, buffer)
for i := 0; i < 100; i++ {
    value := bytesToUint32(buffer[i*4 : i*4+4])
}
3. Cache de Ponteiros
Ponteiros base mudam raramente. Use cache:

var (
    cachedBase     uintptr
    lastCacheTime  time.Time
    cacheDuration  = 50 * time.Millisecond
)

func getBaseCached(handle windows.Handle, x2game uintptr) uintptr {
    if time.Since(lastCacheTime) < cacheDuration && cachedBase != 0 {
        return cachedBase
    }
    
    cachedBase = calculateBase(handle, x2game)
    lastCacheTime = time.Now()
    return cachedBase
}
4. Identificando Strings/Nomes
Nomes geralmente estão atrás de 2 ponteiros:

const (
    OFF_NAME_PTR1 = 0x0C
    OFF_NAME_PTR2 = 0x1C
)

func getEntityName(handle windows.Handle, entityAddr uint32) string {
    ptr1 := readU32(handle, uintptr(entityAddr + OFF_NAME_PTR1))
    if !isValidPtr(ptr1) {
        return ""
    }
    
    ptr2 := readU32(handle, uintptr(ptr1 + OFF_NAME_PTR2))
    if !isValidPtr(ptr2) {
        return ""
    }
    
    return readString(handle, uintptr(ptr2), 32)
}
5. Encontrando Entidades por VTable Scan
Para encontrar TODAS as entidades, escaneie memória procurando VTables conhecidos:

func findAllEntities(handle windows.Handle, player Entity) []Entity {
    var entities []Entity
    
    // Regiões de memória onde entidades ficam
    regions := []struct{ start, size uint32 }{
        {0x80000000, 0x10000000},
        {0x90000000, 0x10000000},
        // ...
    }
    
    buffer := make([]byte, 0x10000)
    
    for _, region := range regions {
        for offset := uint32(0); offset < region.size; offset += 0x10000 {
            addr := region.start + offset
            readMemoryBytes(handle, uintptr(addr), buffer)
            
            // Procura por VTables válidos
            for i := uint32(0); i < 0x10000-0x900; i += 4 {
                vtable := bytesToUint32(buffer[i : i+4])
                
                // VTable na faixa esperada?
                if vtable < 0x39000000 || vtable >= 0x3B000000 {
                    continue
                }
                
                // Valida HP
                hp := bytesToUint32(buffer[i+OFF_HP_ENTITY : i+OFF_HP_ENTITY+4])
                if hp < 100 || hp > 10000000 {
                    continue
                }
                
                // Valida posição
                posX := bytesToFloat32(buffer[i+OFF_POS_X : i+OFF_POS_X+4])
                if !isValidCoord(posX) {
                    continue
                }
                
                // Entidade válida encontrada!
                candidateAddr := addr + i
                // ... adiciona à lista
            }
        }
    }
    
    return entities
}
6. Debugging com Breakpoints
No x64dbg/x32dbg:

Encontre o endereço que acessa o HP
Coloque um Hardware Breakpoint on Access
Quando quebrar, analise os registradores:
EAX = 0x00003D07 (15623 em decimal = HP!)
ESI = 0x1A5F3000 (Entity Address)
[ESI+84C] = HP
7. Atualizações do Jogo
Quando o jogo atualiza, offsets podem mudar. Estratégias:

Pattern Scanning: Procure por padrões de bytes em vez de offsets fixos
Signature Scanning: Use assinaturas de código assembly
Offset Tables: Mantenha offsets em arquivo JSON para fácil atualização
// Pattern scan exemplo
pattern := []byte{0x8B, 0x86, 0x00, 0x00, 0x00, 0x00, 0x85, 0xC0}
mask := "xx????xx"
// 0x8B 0x86 = mov eax, [esi+????]
// O ???? é o offset que queremos encontrar
📊 Tabela de Offsets Atual
Offset Tamanho Descrição Chain 0xE9DC54 ptr LocalPlayer Pointer x2game.dll+ 0x10 ptr Entity Address +0xE9DC54]+ 0x38 ptr EntityBase Entity+ 0x0C ptr Name Pointer 1 Entity+ 0x1C ptr Name Pointer 2 NamePtr1+ 0x830 float Position X Entity+ 0x834 float Position Z Entity+ 0x838 float Position Y Entity+ 0x84C uint32 Current HP Entity+ 0x1898 ptr Debuff Manager EntityBase+ 0x4698 ptr Stats Pointer (ESI) EntityBase+ 0x10 ptr Stats Structure ESI+ 0x420 uint32 Max HP Stats+ 0x20 uint32 Debuff Count DebuffMgr+ 0xD30 array Debuff Array DebuffMgr+
🎓 Recursos Adicionais
Cheat Engine Tutorial Series
Game Hacking Academy
GuidedHacking Forum
UnknownCheats Forum
ReClass.NET Documentation
⚠️ Aviso Legal
Este guia é apenas para fins educacionais. Engenharia reversa pode violar:

Termos de Serviço de jogos
Leis de direitos autorais (DMCA, etc.)
Leis de computação em alguns países
Sempre verifique a legalidade em sua jurisdição antes de aplicar estas técnicas.
