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
    timestep: # uint16
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
modbus-serve-csv [-t timestep] [-T timeout] [-F files...] [--verbose]
```

| flag | type | description |
| ---- | ---- | ----------- |
| timestep | uint16 | Set *global* timestep. See example below. |
| timeout | uint16 | Set automatic timeout
| files | string... | Only simulate these files (files must be contained in config.yaml)
| verbose | | Print current timestep and simulated values (only if global timestep set)

Update frequencies depending on value of supplied -t.

| file | config timestep | flag not set | -t 5 | -t 10 | -t 8
| - | - | - | - | - | - |
| classroom_1.csv | 5 | 5 | 5 | 10 | 8 |
| classroom_2.csv | 5 | 5 | 5 | 10 | 16 |
| classroom_3.csv | 10 | 10 | 10 | 20 | 16 |
| elec.csv | 12 | 12 | 12 | 24 | 19.2 |

All files must have timestep defined in configuration for this mode. Otherwise value of -t will be used for all.
