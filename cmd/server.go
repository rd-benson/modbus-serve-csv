package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/tbrandon/mbserver"
)

type Config struct {
	Filename string `mapstructure:"filename"`
	Port     uint16 `mapstructure:"port"`
	SlaveId  uint8  `mapstructure:"slave-id"`
}

type LoopReader struct {
	filename string
	file     *os.File
	reader   *csv.Reader
	value    []float32
}

func (lr *LoopReader) getReader() {
	f, err := os.Open("./data/" + lr.filename)
	cobra.CheckErr(err)
	lr.file = f
	lr.reader = csv.NewReader(lr.file)
}

func (lr *LoopReader) close() {
	fmt.Printf("closing %v\n", lr.filename)
	lr.file.Close()
}

func (lr *LoopReader) readLine() (record []float32) {
	stringRecord, err := lr.reader.Read()
	if err == io.EOF {
		lr.close()
		lr.getReader()
		return lr.readLine()
	} else if err != nil {
		panic(err)
	}
	for _, s := range stringRecord {
		r, err := strconv.ParseFloat(s, 32)
		cobra.CheckErr(err)
		record = append(record, float32(r))
	}
	return record
}

func (lr *LoopReader) cycle() {
	if lr.reader == nil {
		fmt.Println("got new reader")
		lr.getReader()
		lr.cycle()
	}
	lr.value = lr.readLine()
}

type Simulation struct {
	port   uint16
	id     uint8
	reader LoopReader
	server *mbserver.Server
}

func (s *Simulation) getInitialValue() {
	s.reader.getReader()
	s.reader.value = s.reader.readLine()
}

func (s *Simulation) listen() {
	err := s.server.ListenTCP(fmt.Sprintf("0.0.0.0:%v", s.port))
	cobra.CheckErr(err)
	defer s.server.Close()
	for {
		time.Sleep(1 * time.Second)
	}
}

// Helper constructor method to generate new simulation from config struct
func NewSimulation(c Config) Simulation {
	sim := Simulation{
		port: c.Port,
		id:   c.SlaveId,
		reader: LoopReader{
			filename: c.Filename,
			file:     nil,
			reader:   nil,
			value:    nil,
		},
		server: mbserver.NewServer(),
	}
	sim.getInitialValue()
	return sim
}

type Termination struct {
	interrupt chan os.Signal
	timeout   chan bool
}

// Cycle instructs each simulation to update to the next value in it's file.
// All simulations in s are updated simulataneously
func Cycle(s []Simulation, ticker *time.Ticker, term *Termination) {
	go func() {
		for {
			select {
			case <-term.interrupt:
				Terminate(s, "user interrupt")
				return
			case <-term.timeout:
				Terminate(s, "automatic timeout")
				return
			case <-ticker.C:
				fmt.Print("reading from ... ")
				for i := 0; i < len(s); i++ {
					fmt.Printf("%v, ", s[i].reader.filename)
					s[i].reader.cycle()
				}
				fmt.Println()
			}
		}
	}()
}

func Terminate(s []Simulation, reason string) {
	fmt.Printf("Simulation terminated: %v\n", reason)
	for i := 0; i < len(s); i++ {
		s[i].reader.close()
	}
	os.Exit(0)
}
