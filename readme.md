# 🎮 Muletinha GOTY Edition

Um overlay/assistente para ArcheAge desenvolvido em Go com interface gráfica usando Ebiten.

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![Platform](https://img.shields.io/badge/Platform-Windows-0078D6?style=flat&logo=windows)
![License](https://img.shields.io/badge/License-MIT-green.svg)

## ✨ Funcionalidades

### 🗺️ Radar
- Visualização em tempo real de entidades próximas
- Diferenciação entre Players (vermelho) e NPCs (amarelo)
- Range configurável de 1000 unidades

### �� Auto Potion
- **Desert Fire (F1)**: Poção de cura rápida com cooldown de 1.5s
- **Nui's Nova (F2)**: Poção de emergência com cooldown de 30s
- Thresholds configuráveis via sliders na interface
- Toggle individual para cada poção

### 🛡️ CC Break (Crowd Control)
- Detecção instantânea de debuffs de CC
- Reação automática com spam de teclas configuráveis
- Whitelist customizável via `cc_whitelist.json`
- Suporte a combinações de teclas (SHIFT+1, CTRL+ALT+F1, etc.)

### ⚔️ Buff Break
- Monitoramento de buffs inimigos
- Reação automática para quebrar buffs específicos
- Whitelist customizável via `buff_whitelist.json`

### 📊 Interface
- Barra de HP com indicadores visuais de thresholds
- Lista de buffs/debuffs ativos com tempo restante
- Log de eventos com indicação de reações automáticas
- Painel de configuração com toggles e sliders


## 🚀 Instalação

### Pré-requisitos
- Go 1.21 ou superior
- Windows 10/11
- ArcheAge instalado

### Build

```bash
# Clone o repositório
git clone https://github.com/seu-usuario/muletinha.git
cd muletinha

# Instale as dependências
go mod tidy

# Execute
go run main.go

# Ou compile para executável
go build -ldflags="-H windowsgui" -o muletinha.exe
⚙️ Configuração
cc_whitelist.json
[
  {
    "type": 3601,
    "name": "stun",
    "use": "F12"
  },
  {
    "type": 509,
    "name": "knockdown",
    "use": "SHIFT+F12"
  }
]
buff_whitelist.json
[
  {
    "type": 87,
    "name": "Hell Spear",
    "use": "F10"
  },
  {
    "type": 243,
    "name": "stun",
    "use": "SHIFT+1"
  }
]
Teclas Suportadas
Categoria Teclas Função F1-F12 Números 0-9 Letras A-Z Numpad NUM0-NUM9, NUMPAD0-NUMPAD9 Especiais SPACE, ENTER, TAB, ESC, BACKSPACE Navegação UP, DOWN, LEFT, RIGHT, HOME, END Modificadores SHIFT, CTRL, ALT, LSHIFT, RSHIFT, LCTRL, RCTRL, LALT, RALT
Exemplos de Combinações
F1 - Tecla simples
SHIFT+1 - Shift + número
CTRL+ALT+F1 - Múltiplos modificadores
CTRL+SHIFT+5 - Três teclas
🎮 Hotkeys
Tecla Função F3 Toggle CC Break F4 Toggle Buff Break
📝 Notas
Execute como Administrador para garantir acesso à memória do processo
Os arquivos de whitelist são gerados automaticamente na primeira execução
Os offsets podem mudar com atualizações do jogo
🔧 Dependências
Ebiten v2 - Game library para Go
golang.org/x/sys - Chamadas de sistema Windows
⚠️ Disclaimer
Este projeto é apenas para fins educacionais. O uso de ferramentas de terceiros pode violar os Termos de Serviço do jogo. Use por sua conta e risco.

📄 Licença
MIT License - veja LICENSE para detalhes.