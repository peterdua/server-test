package main

import (
	"flag"
	"log"
	"net"
	"net/rpc"
	"os"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type Worker struct {
	world       [][]uint8
	currentTurn int
	Param       stubs.Params
	mutex       *sync.Mutex
	paused      bool
	pauseChan   chan bool
	quitChan    chan bool
	exitChan    chan bool
}

type workerChannels struct {
	worldSlice  chan [][]uint8
	flippedCell chan []util.Cell
}

func (w *Worker) GameOfLife(req stubs.GameOfLifeRequest, res *stubs.GameOfLifeResponse) error {
	w.Param = req.Params
	w.world = req.World
	w.currentTurn = 0
	for w.currentTurn < req.Params.Turns {
		select {
		case <-w.pauseChan:
			log.Printf("Turn %d paused", w.currentTurn)
			for {
				<-w.pauseChan
				log.Printf("Turn %d resume", w.currentTurn)
				break
			}
		case <-w.quitChan:
			w.world = nil
			res.Turns = w.currentTurn
			w.currentTurn = 0
			return nil
		case <-w.exitChan:
			os.Exit(0)
		default:
			w.mutex.Lock()
			newWorld, _ := CalculateNextState(w.world, req.Params)
			w.currentTurn++
			w.world = newWorld
			w.mutex.Unlock()
		}
	}

	res.World = w.world
	res.Turns = w.currentTurn
	res.AliveCells = calculateAliveCells(req.Params, w.world)

	return nil
}

func (w *Worker) GetAliveCells(req stubs.GetAliveCellsRequest, res *stubs.GetAliveCellsResponse) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	res.Turn = w.currentTurn
	res.AliveCellsCount = len(calculateAliveCells(w.Param, w.world))
	return nil
}

func (w *Worker) KeyPress(req stubs.KeyPressRequest, res *stubs.KeyPressResponse) error {
	res.Turn = w.currentTurn
	switch req.Key {
	case 'p':
		w.pauseChan <- true
		w.paused = !w.paused
		res.Paused = w.paused
	case 'q':
		w.quitChan <- true
		res.Turn = w.currentTurn
		res.World = w.world
		res.AliveCells = calculateAliveCells(w.Param, w.world)
	case 's':
		res.World = w.world
	case 'k':
		res.World = w.world
		w.exitChan <- true
	}

	return nil
}

func makeImmutableMatrix(matrix [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return matrix[y][x]
	}
}

func MakeNewWorld(height, width int) [][]uint8 {
	newWorld := make([][]uint8, height)
	for i := range newWorld {
		newWorld[i] = make([]uint8, width)
	}
	return newWorld
}

func calculateNewCellValue(Y1, Y2, X1, X2 int, data func(y, x int) uint8, p stubs.Params) ([][]uint8, []util.Cell) {
	height := Y2 - Y1
	width := X2 - X1
	nextSLice := MakeNewWorld(height, width)
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

func worker(Y1, Y2, X1, X2 int, data func(y, x int) uint8, out workerChannels, p stubs.Params) {
	work, workCell := calculateNewCellValue(Y1, Y2, X1, X2, data, p)
	out.worldSlice <- work
	out.flippedCell <- workCell
}

func CalculateNextState(world [][]uint8, p stubs.Params) ([][]uint8, []util.Cell) {
	data := makeImmutableMatrix(world)
	var newPixelData [][]uint8
	var flipped []util.Cell
	if p.Threads == 1 {
		newPixelData, flipped = calculateNewCellValue(0, p.ImageHeight, 0, p.ImageWidth, data, p)
	} else {
		ChanSlice := make([]workerChannels, p.Threads)

		for i := 0; i < p.Threads; i++ {
			ChanSlice[i].worldSlice = make(chan [][]uint8)
			ChanSlice[i].flippedCell = make(chan []util.Cell)
		}
		for i := 0; i < p.Threads-1; i++ {
			go worker(int(float32(p.ImageHeight)*(float32(i)/float32(p.Threads))),
				int(float32(p.ImageHeight)*(float32(i+1)/float32(p.Threads))),
				0, p.ImageWidth, data, ChanSlice[i], p)
		}
		go worker(int(float32(p.ImageHeight)*(float32(p.Threads-1)/float32(p.Threads))),
			p.ImageHeight,
			0, p.ImageWidth, data, ChanSlice[p.Threads-1], p)

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

func main() {
	port := flag.String("port", "8080", "port to listen on")
	flag.Parse()

	listener, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	log.Printf("Listening on port %s", *port)

	rpc.Register(&Worker{
		world:       nil,
		Param:       stubs.Params{},
		currentTurn: 0,
		mutex:       &sync.Mutex{},
		paused:      false,
		pauseChan:   make(chan bool),
		quitChan:    make(chan bool),
		exitChan:    make(chan bool),
	})
	rpc.Accept(listener)
}

func calculateAliveCells(p stubs.Params, world [][]byte) []util.Cell {
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
