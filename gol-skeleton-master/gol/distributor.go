package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"os"
	"strconv"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

type workerChannels struct {
	worldSlice  chan [][]uint8
	flippedCell chan []util.Cell
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	wd := p.ImageWidth
	hd := p.ImageHeight
	ChannelClosed := false
	c.ioCommand <- ioInput
	filename1 := fmt.Sprintf("%dx%d", hd, wd)
	c.ioFilename <- filename1

	//  Create a 2D slice to store the world.
	world := MakeNewWorld(hd, wd)
	for i := range world {
		world[i] = make([]byte, wd)
		for j := range world[i] {
			world[i][j] = <-c.ioInput
			if world[i][j] == 255 {
				c.events <- CellFlipped{
					Cell:           util.Cell{X: j, Y: i},
					CompletedTurns: 0,
				}
			}
		}
	}
	turn := 0

	golWorker, err := rpc.Dial("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}
	defer golWorker.Close()

	go timer(golWorker, c.events, &ChannelClosed)
	go keypress(golWorker, p, c, &ChannelClosed)

	var res stubs.GameOfLifeResponse
	req := stubs.GameOfLifeRequest{
		World: world,
		Params: stubs.Params{
			ImageWidth:  p.ImageWidth,
			ImageHeight: p.ImageHeight,
			Turns:       p.Turns,
			Threads:     p.Threads,
		},
	}

	err = golWorker.Call(stubs.GameOfLife, req, &res)
	if err != nil {
		panic(err)
	}

	world = res.World
	turn = res.Turns

	OutPutFile(world, c, p, turn)
	// Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: res.AliveCells}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{turn, Quitting}
	ChannelClosed = true
	close(c.events)
}

func MakeNewWorld(height, width int) [][]uint8 {
	newWorld := make([][]uint8, height)
	for i := range newWorld {
		newWorld[i] = make([]uint8, width)
	}
	return newWorld
}

func timer(golWorker *rpc.Client, eventChan chan<- Event, ChannelClosed *bool) {
	for {
		time.Sleep(time.Second * 2)

		if !*ChannelClosed {
			var res stubs.GetAliveCellsResponse
			err := golWorker.Call(stubs.GetAliveCells, stubs.GetAliveCellsRequest{}, &res)
			if err != nil {
				log.Printf("Error: %v", err)
			}
			eventChan <- AliveCellsCount{CellsCount: res.AliveCellsCount, CompletedTurns: res.Turn}
		}
	}
}

func OutPutFile(world [][]uint8, c distributorChannels, p Params, turn int) {
	HD := strconv.Itoa(p.ImageHeight)
	WD := strconv.Itoa(p.ImageWidth)
	TR := strconv.Itoa(turn)
	c.ioCommand <- ioOutput
	FilenameOut := WD + "x" + HD + "x" + TR
	c.ioFilename <- FilenameOut
	if len(world) == 0 {
		return
	}
	hd := p.ImageHeight
	wd := p.ImageWidth
	for x := 0; x < hd; x++ {
		for y := 0; y < wd; y++ {
			c.ioOutput <- world[x][y]
		}
	}
	c.events <- ImageOutputComplete{
		CompletedTurns: turn,
		Filename:       FilenameOut,
	}
}

func keypress(golWorker *rpc.Client, p Params, c distributorChannels, ChannelClosed *bool) {
	for {
		key := <-c.keyPresses
		var res stubs.KeyPressResponse
		err := golWorker.Call(stubs.KeyPress, stubs.KeyPressRequest{Key: key}, &res)
		if err != nil {
			panic(err)
		}
		switch key {
		case 's':
			fallthrough
		case 'k':
			OutPutFile(res.World, c, p, res.Turn)
			if key == 'k' {
				c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: res.AliveCells}
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- StateChange{res.Turn, Quitting}
				close(c.events)
				os.Exit(0)
			}
		case 'q':
			OutPutFile(res.World, c, p, res.Turn)
			c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: res.AliveCells}
			c.ioCommand <- ioCheckIdle
			<-c.ioIdle
			c.events <- StateChange{res.Turn, Quitting}
			close(c.events)
		case 'p':
			*ChannelClosed = true
			c.events <- StateChange{res.Turn, Paused}
			for {
				Key := <-c.keyPresses
				if Key == 'p' {
					err := golWorker.Call(stubs.KeyPress, stubs.KeyPressRequest{Key: Key}, &res)
					if err != nil {
						panic(err)
					}
					*ChannelClosed = false
					c.events <- StateChange{res.Turn, Executing}
					break
				}
			}
		}
	}
}
