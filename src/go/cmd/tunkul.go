package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ingyamilmolinar/tunkul/internal/ui"
	//"github.com/ingyamilmolinar/tunkul/internal/game"
)

func main() {
    // Create an instance of our game
    // g := game.NewTunkulGame()
    g := ui.New()

    // Optional window settings (not used in WASM, but for desktop builds)
    ebiten.SetWindowSize(640, 480)
    ebiten.SetWindowTitle("Tunkul - Node Music Game")

    // Run the game. On WASM, this will create a <canvas> in index.html
    if err := ebiten.RunGame(g); err != nil {
        log.Fatal(err)
    }
}
