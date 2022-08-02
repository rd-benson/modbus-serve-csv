package cmd

import (
	"bytes"
	"encoding/binary"
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
	RegAddress uint16
	RegType    string
	ByteSwap   bool
	ValueType  string
	byteOrder  binary.ByteOrder
}

func (p *Params) getByteOrder() {
	if p.ByteSwap {
		p.byteOrder = binary.LittleEndian
	} else {
		p.byteOrder = binary.BigEndian
	}
}

var validValueTypes = []string{"bool", "int8", "int16", "uint32", "uint8", "uint16", "uint32", "float32", "float64"}

func newDefaultParams() Params {
	p := Params{
		RegAddress: 0,
		RegType:    "holding",
		ByteSwap:   false,
		ValueType:  "int16",
	}
	p.getByteOrder()
	return p
}

type LoopReader struct {
	filename string
	file     *os.File
	reader   *csv.Reader
	value    [][]byte
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

func parseRecord(byteOrder binary.ByteOrder, valueType string, s string) ([]byte, error) {

	buf := new(bytes.Buffer)
	var err error

	switch valueType {
	// Bool:
	case "bool":
		val, e := strconv.ParseBool(s)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, bool(val))
	// Int
	case "int8":
		val, e := strconv.ParseInt(s, 10, 8)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, int8(val))
	case "int16":
		val, e := strconv.ParseUint(s, 10, 16)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, int16(val))
	case "int32":
		val, e := strconv.ParseUint(s, 10, 32)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, int32(val))
	case "int64":
		val, e := strconv.ParseInt(s, 10, 64)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, int64(val))
	// Uint
	case "uint8":
		val, e := strconv.ParseUint(s, 10, 8)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, uint8(val))
	case "uint16":
		val, e := strconv.ParseUint(s, 10, 16)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, uint16(val))
	case "uint32":
		val, e := strconv.ParseUint(s, 10, 32)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, uint32(val))
	case "uint64":
		val, e := strconv.ParseUint(s, 10, 64)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, uint64(val))
		// Float
	case "float32":
		val, e := strconv.ParseFloat(s, 32)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, float32(val))
	case "float64":
		val, e := strconv.ParseFloat(s, 64)
		cobra.CheckErr(e)
		err = binary.Write(buf, byteOrder, float64(val))
	}
	return buf.Bytes(), err
}

func (lr *LoopReader) readRecord() {
	stringRecord, err := lr.reader.Read()
	if err == io.EOF {
		lr.getReader()
		lr.readRecord()
	} else if err != nil {
		panic(err)
	}
	for i, s := range stringRecord {
		val, err := parseRecord(lr.params[i].byteOrder, lr.params[i].ValueType, s)
		if err == nil {
			lr.value[i] = val
		}
	}
}

func (lr *LoopReader) nextRecord() {
	if lr.reader == nil {
		lr.getReader()
		lr.nextRecord()
	}
	lr.readRecord()
}

type Simulation struct {
	port   uint16
	id     uint8
	reader LoopReader
	server *mbserver.Server
}

func (s *Simulation) update() {
	s.reader.nextRecord()
	updateServer(s)
}

func validateParams(paramsSlice []Params) []Params {
	for i, params := range paramsSlice {
		var errs []error = nil
		switch params.RegType {
		case "coil", "discrete":
			params.ByteSwap = false
			params.ValueType = "bool"
		case "input", "holding":
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
		params.getByteOrder()
		for err := range errs {
			fmt.Printf("Error: %v (defaults will be used instead)\n", err)
		}
		paramsSlice[i] = params
	}

	return paramsSlice
}

// Helper constructor method to generate new simulation from config struct
func NewSimulation(c Config) Simulation {
	params := validateParams(c.Params)
	value := make([][]byte, len(params))
	sim := Simulation{
		port: c.Port,
		id:   c.SlaveId,
		reader: LoopReader{
			filename: c.Filename,
			file:     nil,
			reader:   nil,
			value:    value,
			params:   params,
		},
		server: mbserver.NewServer(),
	}
	sim.reader.getReader()
	sim.reader.readRecord()
	return sim
}

func replaceSubSlice(dst *[]uint16, src []byte, start uint16) {
	data := mbserver.BytesToUint16(src)
	for i := 0; i < len(data); i++ {
		(*dst)[start+uint16(i)] = data[i]
	}
}

func updateServer(s *Simulation) {
	for i, param := range s.reader.params {
		addr := param.RegAddress
		switch param.RegType {
		case "coil":
			s.server.Coils[addr] = s.reader.value[i][0]
		case "discrete":
			s.server.DiscreteInputs[addr] = s.reader.value[i][0]
		case "holding":
			replaceSubSlice(&s.server.HoldingRegisters, s.reader.value[i], addr)
		case "input":
			replaceSubSlice(&s.server.InputRegisters, s.reader.value[i], addr)
		}
	}
}

// Server instructs each simulation to:
// 	- listen on 0.0.0.0:port
// 	- update to the next value in it's file.
// All simulations in s are updated simulataneously
func Serve(s []Simulation, ticker *time.Ticker, term *Termination) {
	go func() {
		for i := 0; i < len(s); i++ {
			err := s[i].server.ListenTCP(fmt.Sprintf("0.0.0.0:%v", s[i].port))
			cobra.CheckErr(err)
		}
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
					s[i].update()
				}
			}
		}
	}()
}

type Termination struct {
	interrupt chan os.Signal
	timeout   chan bool
}

func Terminate(s []Simulation, reason string) {
	fmt.Printf("Simulation terminated: %v\n", reason)
	for i := 0; i < len(s); i++ {
		s[i].reader.close()
		s[i].server.Close()
	}
	os.Exit(0)
}
