# modbus-serve-csv

Simple command line utility to simulate modbus servers on a local network with data from a CSV file. Developed for testing of [pigeon](https://github.com/rd-benson/pigeon) & [parrot](https://github.com/rd-benson/parrot).

## Installation

###Prerequisite tools:
* [Git](https://git-scm.com/)
* [Go](https://golang.org/dl/) - tested for Go >= 1.18

###Fetch from github
```bash
mkdir $HOME/src
cd $HOME/src
git clone https://github.com/rd-benson/modbus-serve-csv.git
cd modbus-serve-csv
go install
```

**Windows users substitute `$HOME` with `$USERPROFILE`**

## Usage
### Configuration
Navigate to folder containing CSVs and create a `config.yaml`. 
Keys are case-insensitive ...
```yaml
servers:
  - filename: # filename with extension
    port: # uint16
    slaveid: # unit8
    hasHeader: # bool
    hasIndex: # bool
    missingrate: # float32
    params:
      - regaddress: # uint16
        regtype: # string
        byteswap: # bool
        valuetype: # string
```

**valuetype** must be one of:
* bool
* (u)int16/32/64,
* float32/64

See an example at [configuration](https://github.com/rd-benson/modbus-serve-csv/blob/main/config.yaml) 

### Running
```bash
modbus-serve-csv [-t timestep] [-T timeout] [-F files] [--verbose]
```

**timestep** controls update frequency (seconds)

**timeout** set automatic timeout

**files** only simulate these files (still requires 
configuration)

**verbose** print current timestep and simulated values
