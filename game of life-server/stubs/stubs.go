package stubs

import "uk.ac.bris.cs/gameoflife/util"

const (
	GameOfLife    = "Worker.GameOfLife"
	GetAliveCells = "Worker.GetAliveCells"
	KeyPress      = "Worker.KeyPress"
)

type Params struct {
	ImageWidth  int
	ImageHeight int
	Turns       int
	Threads     int
}

type GameOfLifeRequest struct {
	World  [][]uint8
	Params Params
}

type GameOfLifeResponse struct {
	World      [][]uint8
	Turns      int
	AliveCells []util.Cell
}

type GetAliveCellsRequest struct {
}

type GetAliveCellsResponse struct {
	Turn            int
	AliveCellsCount int
}

type KeyPressRequest struct {
	Key rune
}

type KeyPressResponse struct {
	World      [][]uint8
	Turn       int
	Paused     bool
	AliveCells []util.Cell
}
