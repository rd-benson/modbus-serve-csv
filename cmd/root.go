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

// var sims Simulations

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "modbus-serve-csv",
	Short: "Simulate modbus servers from CSV data",
	Long: `Simulate modbus servers from CSV data in this directory.
See github.com/rd-benson/modbus-serve-csv for full documentation.`,
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

		runSim(ticker, &termSignal)

		// Terminal printing
		go func() {
			i := 0
			for {
				time.Sleep(t)
				if verbose {
					fmt.Printf("timestep = %d\n", i)
					readValues()
				}
				i += 1
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
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Print current values to terminal.")
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
			Filename:  filename,
			Port:      5000 + uint16(i),
			SlaveId:   1,
			HasHeader: false,
			HasIndex:  false,
			Params:    params,
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
			continue
		}
		simulations = append(simulations, NewSimulation(c))
	}
	fmt.Println("simulations, ", simulations)
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

// Test if a slice contains an element
func stringSliceContains(testElem string, slice []string) bool {
	for _, sliceElem := range slice {
		if testElem == sliceElem {
			return true
		}
	}
	return false
}
