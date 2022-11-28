package stubs

import "uk.ac.bris.cs/gameoflife/util"

var GameOfLife = "Broker.GameOfLife"
var KeyPress = "Broker.KeyPress"
var Next = "Server.Next"

type World struct {
	Width  int
	Height int
	Turns  int
	Grid   [][]bool
}

type GolRequest struct {
	World World
}

type GolResponse struct {
	World World
	Turns int
}

type KeyPressRequest struct {
	Key rune
}

type KeyPressResponse struct {
	World  World
	Turn   int
	Paused bool
}

type NextRequest struct {
	World World
	Cells []util.Cell
}

type NextResponse struct {
	Cells map[util.Cell]bool
}
