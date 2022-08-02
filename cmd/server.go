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
	Filename string
	Port     uint16
	SlaveId  uint8
	Params   []Params
}

type Params struct {
	RegAddress    uint16
	RegType       string
	ByteSwap      bool
	WordSwap      bool
	ValueType     string
	registerCount int
}

var validValueTypes = []string{"bool", "int8", "int16", "uint32", "uint8", "uint16", "uint32", "float32", "float64"}

func getRegisterCount(valueType string) int {
	switch valueType {
	case "bool", "int8", "int16", "uint8", "uint16":
		return 1
	case "float64", "int64", "uint64":
		return 4
	default: // case "float32", "int32", "uint32":
		return 2
	}
}

func newDefaultParams() Params {
	return Params{
		RegAddress:    0,
		RegType:       "holding",
		ByteSwap:      false,
		WordSwap:      false,
		ValueType:     "float32",
		registerCount: getRegisterCount("float32"),
	}
}

type LoopReader struct {
	filename string
	file     *os.File
	reader   *csv.Reader
	value    []any
	params   []Params
}

func (lr *LoopReader) getReader() {
	f, err := os.Open(lr.filename)
	cobra.CheckErr(err)
	lr.file = f
	lr.reader = csv.NewReader(lr.file)
}

func (lr *LoopReader) close() {
	lr.file.Close()
}

func parseRecord(valueType string, s string) (any, error) {
	switch valueType {
	case "bool":
		return strconv.ParseBool(s)
	case "int8":
		return strconv.ParseInt(s, 10, 8)
	case "int16":
		return strconv.ParseInt(s, 10, 16)
	case "int32":
		return strconv.ParseInt(s, 10, 32)
	case "int64":
		return strconv.ParseInt(s, 10, 32)
	case "uint8":
		return strconv.ParseUint(s, 10, 8)
	case "uint16":
		return strconv.ParseUint(s, 10, 16)
	case "uint32":
		return strconv.ParseUint(s, 10, 32)
	case "uint64":
		return strconv.ParseUint(s, 10, 32)
	case "float64":
		return strconv.ParseFloat(s, 64)
	default:
		return strconv.ParseFloat(s, 32)
	}
}

func (lr *LoopReader) nextRecord() {
	stringRecord, err := lr.reader.Read()
	if err == io.EOF {
		lr.getReader()
		lr.nextRecord()
	} else if err != nil {
		panic(err)
	}
	if len(stringRecord) == 1 {
		val, err := parseRecord(lr.params[0].ValueType, stringRecord[0])
		cobra.CheckErr(err)
		lr.value = []any{val}
	} else {
		for i, s := range stringRecord {
			val, err := parseRecord(lr.params[i].ValueType, s)
			if err == nil {
				lr.value[i] = val
			}
		}
	}
}

func (lr *LoopReader) cycle() {
	if lr.reader == nil {
		lr.getReader()
		lr.cycle()
	}
	lr.nextRecord()
}

type Simulation struct {
	port   uint16
	id     uint8
	reader LoopReader
	server *mbserver.Server
}

func validateParams(paramsSlice []Params) []Params {
	for i, params := range paramsSlice {
		var errs []error = nil
		switch params.RegType {
		case "coil", "discrete":
			params.ByteSwap = false
			params.WordSwap = false
			params.ValueType = "bool"
		case "input", "holding":
			params.RegType = "input"
			if !stringSliceContains(params.ValueType, validValueTypes) {
				errs = append(errs, fmt.Errorf("unrecognised value type"))
				params.ValueType = "float32"
			}
		default:
			errs = append(errs, fmt.Errorf("unrecognised register type"))
			if !stringSliceContains(params.ValueType, validValueTypes) {
				errs = append(errs, fmt.Errorf("unrecognised value type"))
				params.ValueType = "float32"
			}
		}
		params.registerCount = getRegisterCount(params.ValueType)
		for err := range errs {
			fmt.Printf("Error: %v (defaults will be used instead)\n", err)
		}
		paramsSlice[i] = params
	}

	return paramsSlice
}

/* func (s *Simulation) listen() {
	err := s.server.ListenTCP(fmt.Sprintf("0.0.0.0:%v", s.port))
	cobra.CheckErr(err)
	defer s.server.Close()
	for {
		time.Sleep(1 * time.Second)
	}
} */

// Helper constructor method to generate new simulation from config struct
func NewSimulation(c Config) Simulation {
	params := validateParams(c.Params)
	sim := Simulation{
		port: c.Port,
		id:   c.SlaveId,
		reader: LoopReader{
			filename: c.Filename,
			file:     nil,
			reader:   nil,
			value:    nil,
			params:   params,
		},
		server: mbserver.NewServer(),
	}
	sim.reader.getReader()
	sim.reader.nextRecord()
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
				for i := 0; i < len(s); i++ {
					s[i].reader.cycle()
				}
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
