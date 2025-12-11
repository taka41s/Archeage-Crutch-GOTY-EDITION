package main

import (
	"fmt"
	"muletinha/config"
	"muletinha/game"
	"muletinha/input"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/sys/windows"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if err := input.InitVirtualKeyboard(); err != nil {
		fmt.Printf("[Input] Interception não disponível: %v\n", err)
		fmt.Println("[Input] Usando SendInput como fallback (pode interferir com teclado do usuário)")
	} else {
		fmt.Println("[Input] Interception inicializado - inputs isolados do teclado físico")
	}
	defer input.CloseVirtualKeyboard()

	ebiten.SetWindowSize(config.SCREEN_WIDTH, config.SCREEN_HEIGHT)
	ebiten.SetWindowTitle("Muletinha GOTY Edition")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	ebiten.SetVsyncEnabled(true)

	g := game.NewGame()

	if err := ebiten.RunGame(g); err != nil {
		fmt.Println("Erro:", err)
	}

	if handle := g.GetHandle(); handle != 0 {
		windows.CloseHandle(handle)
	}
}