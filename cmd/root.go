/*
Copyright © 2022 Ritchie+Daffin <dan.b@ritchiedaffin.com>
*/
package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const defaultCfgFile = "config"

var cfg AppConfig
var simulations []Simulation
var timestep, timeout uint16
var verbose bool
var totalTicks uint64

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "modbus-serve-csv",
	Short: "Simulate modbus servers from CSV data",
	Long: `Simulate modbus servers from CSV data in this directory.
See github.com/rd-benson/modbus-serve-csv for full documentation.`,
	Example: "modbus-serve-csv [-T timestep] [-F files...]",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		// Read in the configuration
		viper.Unmarshal(&cfg)
		files, err := cmd.Flags().GetStringSlice("files")
		cobra.CheckErr(err)
		timestepSet := cmd.Flag("timestep").Changed
		// Set up servers for simulation (incl. comfig validation)
		baseTick := initSim(cfg, files, timestepSet)

		// Set up termination channels (user cancellation on CTRL-C or timeout)
		termSignal := Termination{
			interrupt: make(chan os.Signal, 1),
			timeout:   make(chan bool),
		}
		signal.Notify(termSignal.interrupt, syscall.SIGINT, syscall.SIGTERM)

		// Ticker channel controls updating records
		t, err := time.ParseDuration(fmt.Sprintf("%fns", baseTick))
		cobra.CheckErr(err)

		ticker := time.NewTicker(t)
		runSim(ticker, &termSignal)
		// Terminal printing
		pt, err := time.ParseDuration(fmt.Sprintf("%ds", timestep))
		cobra.CheckErr(err)
		go func() {
			for {
				time.Sleep(pt)
				if verbose {
					fmt.Printf("timestep = %d\n", totalTicks)
					readValues()
				}
			}
		}()

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
	rootCmd.PersistentFlags().Uint16VarP(&timestep, "timestep", "t", 30, "Simulation timestep in seconds")
	rootCmd.PersistentFlags().Uint16VarP(&timeout, "timeout", "T", 1, "Automatic timeout period in hours.")
	rootCmd.PersistentFlags().StringSliceP("files", "F", nil, "Simulate only supplied files.")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Print current values to terminal. Only if global timestep set.")
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
		// Use configuration defaults (if CSVs in directory)
		// Check for CSVs
		CSVs := findByExt("./", ".csv")
		if len(CSVs) == 0 {
			panic("no CSVs here!")
		}
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
			Filename:    filename,
			Port:        5000 + uint16(i),
			SlaveId:     1,
			HasHeader:   false,
			HasIndex:    false,
			MissingRate: 0,
			Params:      params,
		})
	}
	return rC
}

// initSim reads configuration file and initiates Simulation instances
func initSim(cfg AppConfig, files []string, timestepSet bool) float64 {
	var configTimesteps []uint16
	for _, c := range cfg.Configs {
		// If files flag, only add those files to simulationb
		if len(files) != 0 && !contains(c.Filename, files) {
			continue
		}
		// Check 0 < missingRate < 1
		if c.MissingRate < 0 || c.MissingRate >= 1 {
			fmt.Printf("MissingRate error (%v)! Require 0<= MissingRate < 1. Setting to 0.", c.Filename)
			c.MissingRate = 0
		}
		configTimesteps = append(configTimesteps, c.Timestep)
		simulations = append(simulations, NewSimulation(c))
	}
	if contains(uint16(0), configTimesteps) {
		for i := 0; i < len(simulations); i++ {
			simulations[i].baseTickMultiplier = 1
		}
		fmt.Printf("Timestep not defined for all files. All simulation will update at an interval of %ds.", timestep)
		return 1e9 * float64(timestep)
	}
	// Check if only one file provided, if so set baseTickMultipler to 1 and return timestep
	if len(simulations) == 1 {
		simulations[0].baseTickMultiplier = 1
		return 1e9 * float64(timestep)
	}
	// Assign baseTickMultiplier to each simulation
	// First, get greatest common denominator
	var GCD uint16 = configTimesteps[0]
	if len(configTimesteps) != 1 {
		GCD = GCDSlice(configTimesteps)
	}
	// Then divide all baseTickMultiplier by the GCD
	for i := 0; i < len(simulations); i++ {
		simulations[i].baseTickMultiplier /= GCD
	}
	// Return baseTick = GCD * timestep / min(configTimesteps) in nanoseconds
	min := minSliceUint(configTimesteps)
	return 1e9 * float64(GCD*timestep) / float64(min)
}

// run the simulation
func runSim(ticker *time.Ticker, termSignal *Termination) {
	fmt.Printf("Starting simulation (timestep=%ds) of ...\n", timestep)
	fmt.Printf("Automatic timeout after %dh, or CTRL-C to end simulation.\n", timeout)
	// Cycle(simulations, ticker, termSignal)
	Serve(simulations, ticker, termSignal)
	fmt.Println("Simulation started")
}

// Read values helper
func readValues() {
	for i := 0; i < len(simulations); i++ {
		fmt.Printf("%v: %v\t", simulations[i].reader.filename, simulations[i].reader.value)
	}
	fmt.Println()
}
