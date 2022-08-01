package cmd

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tbrandon/mbserver"
)

type Config struct {
	Filename string `mapstructure:"filename"`
	Port     uint16 `mapstructure:"port"`
	SlaveId  uint8  `mapstructure:"slave-id"`
}

type Params struct {
	registerAddress uint16
	registerType    uint8
	byteSwap        bool
	wordSwap        bool
	valueType       string
	registerCount   int
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
		registerAddress: 0,
		registerType:    2,
		byteSwap:        false,
		wordSwap:        false,
		valueType:       "float32",
		registerCount:   getRegisterCount("float32"),
	}
}

func parseHeader(header []string) (Params, []error) {
	params := newDefaultParams()
	var errs []error = nil
	if len(header) != 5 {
		return params, errs
	}
	for i, param := range header {
		switch i {
		case 0:
			p, err := strconv.ParseUint(param, 10, 16)
			if err != nil {
				errs = append(errs, err)
				break
			}
			params.registerAddress = uint16(p)
		case 1:
			p, err := strconv.ParseUint(param, 10, 8)
			if err != nil {
				errs = append(errs, err)
				break
			}
			if p > 3 {
				errs = append(errs, errors.New("registerType"))
				break
			}
			params.registerType = uint8(p)
		case 2:
			p, err := strconv.ParseBool(param)
			if err != nil {
				errs = append(errs, err)
				break
			}
			params.byteSwap = p
		case 3:
			p, err := strconv.ParseBool(param)
			if err != nil {
				errs = append(errs, err)
				break
			}
			params.wordSwap = p
		case 4:
			if !stringSliceContains(param, validValueTypes) {
				errs = append(errs, errors.New("valueType"))
				break
			}
			params.valueType = param
		}
	}
	params.registerCount = getRegisterCount(params.valueType)
	return params, errs
}

type LoopReader struct {
	filename string
	file     *os.File
	reader   *csv.Reader
	value    []any
	params   []Params
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

func (lr *LoopReader) parseParams() {
	stringRecord, err := lr.reader.Read()
	if err == io.EOF {
		lr.close()
		panic(fmt.Errorf("no data in %v", lr.filename))
	} else if err != nil {
		panic(err)
	}
	for _, s := range stringRecord {
		p, err := parseHeader(strings.Split(s, "|"))
		if err != nil {
			fmt.Printf("Error! Check header (%v)", lr.filename)
		}
		lr.params = append(lr.params, p)
	}
	fmt.Printf("Parsed params for %v: %v\n", lr.filename, lr.params)
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

func (lr *LoopReader) updateRecord() {
	stringRecord, err := lr.reader.Read()
	if err == io.EOF {
		lr.getReader()
		lr.parseParams()
		lr.updateRecord()
	} else if err != nil {
		panic(err)
	}
	if len(stringRecord) == 1 {
		val, err := parseRecord(lr.params[0].valueType, stringRecord[0])
		cobra.CheckErr(err)
		lr.value = []any{val}
	} else {
		for i, s := range stringRecord {
			val, err := parseRecord(lr.params[i].valueType, s)
			if err == nil {
				lr.value[i] = val
			}
		}
	}
}

func (lr *LoopReader) cycle() {
	if lr.reader == nil {
		lr.getReader()
		lr.parseParams()
		lr.cycle()
	}
	lr.updateRecord()
}

type Simulation struct {
	port   uint16
	id     uint8
	reader LoopReader
	server *mbserver.Server
}

func (s *Simulation) getInitialValue() {
	s.reader.getReader()
	s.reader.parseParams()
	s.reader.updateRecord()
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
