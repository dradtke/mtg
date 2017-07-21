package main

import (
	"fmt"
	"os"

	"github.com/dradtke/mtg"
)

func main() {
	f, err := os.Open("../testdata/hou.dec")
	if err != nil {
		panic(err)
	}

	deck, err := mtg.NewDeck(f)
	if err != nil {
		panic(err)
	}

	fmt.Println("Finished parsing deck:")
	fmt.Println("----------------------")
	fmt.Printf("  %d cards\n", deck.Size())
	_, landCount := deck.Lands()
	fmt.Printf("   - %d lands\n", landCount)

	colors := deck.Colors()
	fmt.Printf("  %d colors: %q\n", len(colors), colors)
}
