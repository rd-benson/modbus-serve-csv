/*
Copyright © 2022 Ritchie+Daffin <dan.b@ritchiedaffin.com>

*/
package cmd

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const defaultCfgFile = "config"

var cfg AppConfig
var simulations []Simulation
var timestep uint16
var timeout uint16

// var sims Simulations

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "modbus-serve-csv",
	Short:   "Simulate modbus servers from CSV data",
	Long:    `See github.com/rd-benson/modbus-serve-csv for full documentation.`,
	Example: "modbus-serve-csv [-T timestep] [-F files...]",
	Run: func(cmd *cobra.Command, args []string) {
		// Read in the configuration
		viper.Unmarshal(&cfg)
		files, err := cmd.Flags().GetStringSlice("files")
		cobra.CheckErr(err)
		// Set up servers for simulation (incl. comfig validation)
		initSim(cfg, files)

		// Ticker channel controls updating records
		t, err := time.ParseDuration(fmt.Sprintf("%ds", timestep))
		cobra.CheckErr(err)
		ticker := time.NewTicker(t)

		termSignal := Termination{
			interrupt: make(chan os.Signal, 1),
			timeout:   make(chan bool),
		}
		signal.Notify(termSignal.interrupt, syscall.SIGINT, syscall.SIGTERM)

		sim := runSim(ticker, &termSignal)

		go func() {
			for {
				time.Sleep(500 * time.Millisecond)
				readValues()
			}
		}()

		sim.Wait()
		// Automatic timeout
		T, err := time.ParseDuration(fmt.Sprintf("%dh", timeout))
		cobra.CheckErr(err)
		time.Sleep(T)
		ticker.Stop()
		termSignal.timeout <- true
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().Uint16VarP(&timestep, "timestep", "t", 1, "Set simulation timestep.")
	rootCmd.PersistentFlags().Uint16VarP(&timeout, "timeout", "T", 1, "Set simulation timestep.")
	rootCmd.PersistentFlags().StringSliceP("files", "F", nil, "Simulate given files.")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {

	wd, err := os.Getwd()
	cobra.CheckErr(err)

	viper.AddConfigPath(wd)
	viper.SetConfigType("yaml")
	viper.SetConfigName(defaultCfgFile)

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		// Use configuration defaults
		RC := createDefaultConfiguration()
		viper.Set("servers", RC)
		viper.SafeWriteConfig()
	}
}

type AppConfig struct {
	Configs []Config `mapstructure:"servers"`
}

// Create default configuration file
func createDefaultConfiguration() []Config {
	rC := []Config{}
	// Get CSVs in folder
	CSVs := findByExt("./", ".csv")
	for i, filename := range CSVs {
		// Read first line of csv to get number of columns
		f, err := os.Open(filename)
		cobra.CheckErr(err)
		defer f.Close()
		reader := csv.NewReader(f)
		records, err := reader.Read()
		cobra.CheckErr(err)
		numCols := len(records)
		// Populate default params
		params := []Params{}
		for i := 0; i < numCols; i++ {
			params = append(params, newDefaultParams())
		}
		// Create default configuration for each CSV
		rC = append(rC, Config{
			Filename: filename,
			Port:     5000 + uint16(i),
			SlaveId:  1,
			Params:   params,
		})
	}
	return rC
}

// Find files of type ext in root directory
func findByExt(root, ext string) []string {
	var a []string
	filepath.WalkDir(root, func(s string, d fs.DirEntry, err error) error {
		cobra.CheckErr(err)
		if filepath.Ext(d.Name()) == ext {
			s, err := filepath.Rel(root, s)
			cobra.CheckErr(err)
			a = append(a, s)
		}
		return nil
	})
	return a
}

// initSim reads configuration file and initiates Simulation instances
func initSim(cfg AppConfig, files []string) {
	for _, c := range cfg.Configs {
		if len(files) != 0 && !stringSliceContains(c.Filename, files) {
			break
		}
		simulations = append(simulations, NewSimulation(c))
	}
}

// run the simulation
func runSim(ticker *time.Ticker, termSignal *Termination) *sync.WaitGroup {
	sim := new(sync.WaitGroup)
	sim.Add(len(simulations))
	fmt.Printf("Starting simulation (timestep=%ds) of ...\n", timestep)
	fmt.Printf("Automatic timeout after %dh, or CTRL-C to end simulation.\n", timeout)
	Cycle(simulations, ticker, termSignal)
	fmt.Println("Simulation started")
	return sim
}

// Read values helper
func readValues() {
	for i := 0; i < len(simulations); i++ {
		fmt.Printf("%v: %v\t", simulations[i].reader.filename, simulations[i].reader.value)
	}
	fmt.Println()
}

// Test if a slice contains an element
func stringSliceContains(testElem string, slice []string) bool {
	for _, sliceElem := range slice {
		if testElem == sliceElem {
			return true
		}
	}
	return false
}
