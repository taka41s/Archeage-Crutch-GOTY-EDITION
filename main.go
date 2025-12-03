package main

import (
	"fmt"
	"muletinha/config"
	"muletinha/game"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/sys/windows"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

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