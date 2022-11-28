package gol

import (
	"fmt"
	"strconv"
	"sync"
	"time"
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

var (
	countGuard sync.Mutex
)

type workerChannels struct {
	worldSlice  chan [][]byte
	flippedCell chan []util.Cell
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	wd := p.ImageWidth  // Defining the image Width
	hd := p.ImageHeight // Defining the image Height
	// Define channel closed (if false means channel open, if true means channel closed)
	ChannelClosed := false
	// Let the IO start input
	c.ioCommand <- ioInput
	// send to the io goroutine and the filename specified by the width and height
	filename1 := fmt.Sprintf("%dx%d", hd, wd)
	c.ioFilename <- filename1

	// TODO: Create a 2D slice to store the world.
	world := MakeWorld(hd, wd) //Create world
	for i := range world {
		world[i] = make([]byte, wd)
		for j := range world[i] {
			world[i][j] = <-c.ioInput
			if world[i][j] == 255 {
				c.events <- CellFlipped{
					// Sends CellFlipped events to notify the GUI about a change of state of a cell
					Cell:           util.Cell{X: j, Y: i},
					CompletedTurns: 0,
				}
			}
		}
	}
	turn := 0

	// TODO: Execute all turns of the Game of Life.
	// set the timer, report the number of cells that are still alive every 2 seconds.
	go timer(p, &world, &turn, c, &ChannelClosed)
	// creat keyboard control
	go keypress(&turn, world, p, c)
	// execute all turn
	for i := 1; i <= p.Turns; i++ {
		nextWorld, flipped := CalculateNextState(world, p)
		for _, cell := range flipped {
			c.events <- CellFlipped{
				CompletedTurns: turn,
				Cell:           cell,
			}
		}
		// Writing operation is locked.
		countGuard.Lock()
		c.events <- TurnComplete{CompletedTurns: turn}
		// Writing operation is unlocked.
		countGuard.Unlock()
		// Writing operation is locked.
		countGuard.Lock()
		turn++
		world = nextWorld
		// Writing operation is unlocked.
		countGuard.Unlock()

	}
	// Output file
	OutPutFile(world, c, p, turn)
	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, world)}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	ChannelClosed = true // close the channel
	close(c.events)
	//os.Exit(2)
}

func makeImmutableMatrix(matrix [][]byte) func(y, x int) byte {
	return func(y, x int) byte {
		return matrix[y][x]
	}
}

// MakeWorld Returns the created 2D slice
func MakeWorld(height int, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func CalculateNextState(world [][]byte, p Params) ([][]byte, []util.Cell) {
	data := makeImmutableMatrix(world)
	var newPixelData [][]byte
	var flipped []util.Cell
	if p.Threads == 1 {
		newPixelData, flipped = calculateNewCellValue(0, p.ImageHeight, 0, p.ImageWidth, data, p)
	} else {
		ChanSlice := make([]workerChannels, p.Threads)

		for i := 0; i < p.Threads; i++ {
			ChanSlice[i].worldSlice = make(chan [][]byte)
			ChanSlice[i].flippedCell = make(chan []util.Cell)
		}
		for i := 0; i < p.Threads-1; i++ {
			go worker(
				int(float32(p.ImageHeight)*(float32(i)/float32(p.Threads))),
				int(float32(p.ImageHeight)*(float32(i+1)/float32(p.Threads))),
				0,
				p.ImageWidth,
				data,
				ChanSlice[i],
				p,
			)
		}
		go worker(
			int(float32(p.ImageHeight)*(float32(p.Threads-1)/float32(p.Threads))),
			p.ImageHeight,
			0,
			p.ImageWidth,
			data,
			ChanSlice[p.Threads-1],
			p,
		)

		makeImmutableMatrix(newPixelData)
		for i := 0; i < p.Threads; i++ {

			part := <-ChanSlice[i].worldSlice
			newPixelData = append(newPixelData, part...)

			flippedPart := <-ChanSlice[i].flippedCell
			flipped = append(flipped, flippedPart...)
		}
	}
	return newPixelData, flipped
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var list []util.Cell
	for n := 0; n < p.ImageHeight; n++ {
		for i := 0; i < p.ImageWidth; i++ {
			if world[n][i] == 255 {
				list = append(list, util.Cell{X: i, Y: n})
			}
		}
	}
	return list
}

// Computes the value of a particular cell based on its neighbours
func calculateNewCellValue(Y1, Y2, X1, X2 int, data func(y, x int) byte, p Params) ([][]byte, []util.Cell) {
	height := Y2 - Y1
	width := X2 - X1
	nextSLice := MakeWorld(height, width)
	var Cell []util.Cell
	for i := Y1; i < Y2; i++ {
		for j := X1; j < X2; j++ {
			alive := 0
			for _, a := range [3]int{j - 1, j, j + 1} {
				for _, q := range [3]int{i - 1, i, i + 1} {
					newK := (q + p.ImageHeight) % p.ImageHeight
					newL := (a + p.ImageWidth) % p.ImageWidth
					if data(newK, newL) == 255 {
						alive++
					}
				}
			}
			if data(i, j) == 255 {
				alive -= 1
				if alive < 2 {
					nextSLice[i-Y1][j-X1] = 0
					cell := util.Cell{X: j, Y: i}
					Cell = append(Cell, cell)
				} else if alive > 3 {
					nextSLice[i-Y1][j-X1] = 0
					cell := util.Cell{X: j, Y: i}
					Cell = append(Cell, cell)
				} else {
					nextSLice[i-Y1][j-X1] = 255
				}
			} else {
				if alive == 3 {
					nextSLice[i-Y1][j-X1] = 255
					cell := util.Cell{X: j, Y: i}
					Cell = append(Cell, cell)
				} else {
					nextSLice[i-Y1][j-X1] = 0
				}
			}
		}
	}
	return nextSLice, Cell
}

// Function used for splitting work between multiple threads
// worker makes a "calculateNewCellValue" call
func worker(Y1, Y2, X1, X2 int, data func(y, x int) byte, out workerChannels, p Params) {
	work, workCell := calculateNewCellValue(Y1, Y2, X1, X2, data, p)
	out.worldSlice <- work
	out.flippedCell <- workCell
}
func timer(p Params, world *[][]byte, turn *int, c distributorChannels, ChannelClosed *bool) {
	for {
		time.Sleep(time.Second * 2)

		if !*ChannelClosed {
			countGuard.Lock()
			// Writing operation is locked.
			number := len(calculateAliveCells(p, *world))
			// Writing operation is unlocked.
			countGuard.Unlock()
			c.events <- AliveCellsCount{CellsCount: number, CompletedTurns: *turn}

		} else {
			return
		}
	}
}
func OutPutFile(world [][]byte, c distributorChannels, p Params, turn int) {
	HD := strconv.Itoa(p.ImageHeight)
	WD := strconv.Itoa(p.ImageWidth)
	TR := strconv.Itoa(turn)
	countGuard.Lock()
	c.ioCommand <- ioOutput
	FilenameOut := WD + "X" + HD + "X" + TR
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
	countGuard.Unlock()
}
func keypress(turn *int, world [][]byte, p Params, c distributorChannels) {
	for {
		key := <-c.keyPresses
		switch key {
		case 's':
			// Press s, generate a PGM file with the current state of the board.
			OutPutFile(world, c, p, *turn)
		case 'q':
			// press q, generate a PGM file with the current state of the board and then terminate the program.
			// Your program should not continue to execute all turns set in
			{
				OutPutFile(world, c, p, *turn)

				c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, world)}
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- StateChange{*turn, Quitting}
				close(c.events)
				//os.Exit(2)
			}
		case 'p':
			// press p, pause the processing and print the current turn that is being processed.
			// If p is pressed again resume the processing and print "Continuing".
			// It is not necessary for q and s to work while the execution is paused.
			// Writing operation is locked.
			countGuard.Lock()
			c.events <- StateChange{*turn, Paused}
			for {
				Key := <-c.keyPresses
				if Key == 'p' {
					break
				}
			}
			// Writing operation is locked.
			countGuard.Unlock()
			c.events <- StateChange{*turn, Executing}
		}
	}
}
