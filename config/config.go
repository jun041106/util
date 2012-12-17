// Copyright 2012 Apcera Inc. All rights reserved.

/*
	Package config implements command-line and/or configuration file parsing.
	
	All parameters in the command line and configuration file are parsed
	into values of fields in the struct provided to parse functions.
	Configuration file is a JSON file. Command line parameters are defined
	by tags specified for struct fields. If same parameter is specified in
	the command line and in the configuration file then the value from the
	command line is used.
	
	The field tags are coded with the "conf" name as:
	
	  	Name  type   `conf:"tag1,tag2,..."`
	  
	A special tag "-" directs this package to ignore the field:

	  	Name  type   `conf:"-"`
	  	
	There are boolean tags that have no value and tags that require a value
	specified as "tagName=value". Tags may be specified in any order.
	List of tags:
	
		// Specifies one or more names of the parameter on the command-line.
		// Names are separated by the | character. Fields that do not specify
		// this tag are not allowed on the command line unless
		// Options.AllowAllCmdLine is set to true.
		cmd=name[|name2...]
		
		// Specifies default value of the field. Field value is set
		// to the default value if it is not specified on the command line
		// or in the configuration file.
		default=value
		
		// Specifies that this command line parameter defines the name
		// of the configuration file. For example, often the configuration
		// struct has the field:
		//      ConfigFile   `conf:"cmd=c|config,configfile"`
		// Only one field in the configuration struct may specify this tag.
		configfile
		
		// Boolean tag specifying that the parameter is mandatory.
		// Mandatory parameters must be specified on the command line
		// or in the configuration file. Mandatory fields must not specify
		// default value.
		must
		
	For exampe, to describe an integer parameter Port that can be entered
	as -p or -port on the command line, with default value of 1234,	the field
	is desribed as:
	
	    Port  int  `conf:"cmd=p|port,default=1234"`
	    
	In the configuration file such parameter must have the name "Port".
	
	Fields in the configuration struct that can be specified via either a
	command line parameters or in the configuration file may be of the
	following types:
	
		string
		bool
		int, int8, int16, int32, int64	    
		uint, uint8, uint16, uint32, uint64	    
		float32, float64
		time.Duration
		
	Values of primitive types are allowed in the same format as documented
	in the "flag" package. Additionally, base-10 integer values are accepted
	in a comma-separated format, such as "1,000,000" or "-12,123". Anonymous
	struct fields are allowed and treated as if their fields are included
	directly into the top-level struct.
		
	Fields of the following types can be read only from the configuration file:

		named struct field	
		map[string]interface{}	
		slice of any legal type above
		
	Parameters on the command line can be specified same way as documented
	in the Go "flag" package:
	
		-name
		-name=value			
		-name value		// only non-bool parameters
	    		
*/
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const confTagsName = "conf"

// ValueSource specifies the source of the value set into the configuration
// field.
type ValueSource int
const (
	// Parameter value was not set. This means the parameter was not
	// specified on the command line or in the configuration file,
	// nor it has a default value.
	NotSet			ValueSource = iota

	// Parameter value was set from specified default value.
	// This means the parameter was not specified on the command line
	// or in the configuration file.
	FromDefault

	// Parameter value was set from the command line.
	FromCmdLine

	// Parameter value was set from the configuration file. If command line
	// was parsed this means the parameter was not specified on the command
	// line but was specified in the configuration file.
	FromConfigFile
)

func (v ValueSource) String() string {
	switch v {
	case NotSet:
		return "NotSet"
	case FromDefault:
		return "FromDefault"
	case FromCmdLine:
		return "FromCmdLine"
	case FromConfigFile:
		return "FromConfigFile"
	}
	return "Invalid"
}

// ErrorHandling specifies how to process errors in the command line
// or configuration file parameters. ConfigOptions allows to configure
// reaction to errors in the command line and configuration file separately.
// By default both are set to ExitOnError, i.e. the program will exit on any
// encountered error. Notice this does not affect programmatic errors that may
// be returned if the configuration structure contains errors, for example
// two fields declare same name of the command-line parameter, have invalid
// default value, etc.
type ErrorHandling int
const (
	// Print error message and exit on any error.
	ExitOnError ErrorHandling = iota
	
	// If error is encountered it is returned programmatically by the
	// Parse...() functions.
	ReturnError
	
	// Panic on any error. This may be useful in some cases, for example,
	// when the set of parameters or a configuration file were generated
	// by the program and no errors are expected.
	PanicOnError
)

type ConfigOptions struct {

	// Specifies default config file name. May remain empty if no
	// default config file should be parsed.
	DefaultConfigFile	string

	// If true and the default config file name is specified then
	// it must exist. By default only config file explicitly specified
	// on the command line must exist.
	ConfigFileMustExist	bool
	
	// Specifies reaction to program errors in the config structure.
	// Program errors are, for example, if two fields specify the same
	// name of the command line parameter, or if the default field value
	// is invalid, and similar.  
	DataError			ErrorHandling

	// Specifies reaction to errors in the configuration file.  
	ConfigFileError		ErrorHandling

	// Specifies reaction to errors in the command line parameters.  
	CmdLineError		ErrorHandling

	// If true then all fields in the configuration structure may be
	// specified on the command line. Command-line parameter may be
	// specified same as the name of the field or by any name specieid
	// in the "cmd" tag. If this remain false then only fields with the
	// "cmd" tag can be specified on the command line only via names
	// specified in the "cmd" tag.
	AllowAllCmdLine		bool
	
	// If true then all trailing extra non-parameter values are ignored and
	// collected in remaining params. By default the command line can
	// contain only valid parameters and their values, presence of any
	// extra values is an error.
	AllowCmdLineTrailingExtra	bool
	
	// If true then all extra non-parameter values, before or after valid
	// parameters and their values, are ignored and collected in remaining
	// params. If this is set to true it superceeds AllowCmdLineTrailingExtra.
	AllowCmdLineAnyExtra	bool
	
	// If true then double dash is allowed on the command line. All 
	// parameters after the double dash are placed in remaining params.
	AllowCmdLineDoubleDash	bool
	
	// If true then parameters '-h' and '-help' are not automatically
	// processed as a help request.  
	DisableHelpParams	bool
	
	// Help printed into the console when help is requested via
	// command-line parameters -h or -help.
	Help				string

	// Optionally specifies the command line arguments to parse.
	// If this is nil then parameters are parsed from os.Args.
	// This allows to override using os.Args.
	Args				[]string
}

// Contains information about parsed configuration.
type Config struct {

	// Returned error is a programmatic error, for example in the
	// configuration struct field tags or similar.
	ProgramError	bool

	// Returned error occured when parsing the command line parameters.
	CmdLineError	bool

	// Returned error occured when parsing the configuration file.
	ConfigFileError	bool

	options			ConfigOptions			// copy of user's options
	args			[]string				// from user or os.Args from 1
	remaining		[]string
	
	parseCmdLine	bool					// need to parse cmd line
	parseConfigFile	bool					// parse config file if specified
	canHaveExtra	bool					// if any Extra option=true
	configFile		string					// value from cmd line					
	ncfgfile		int						// track configfile tags
	readFile		string					// what we actually read, if any

	fields			map[string]*fieldInfo	// mapped by each name
	
	helpRequested	bool
	doubleDash		bool
	
	config			reflect.Value
}

// Returns source of the value set into the configuration field.
// Parameter must be the name of the field in the conifuration struct,
// not the name(s) used in the command line. If specified name is not
// a valid field in the configuration struct this function returns NotSet. 
func (cfg *Config) GetValueSource(fieldName string) ValueSource {
	fi := cfg.fields[fieldName]
	if fi != nil {
		return fi.valueSource
	}
	return NotSet	
}

// Returns command line parameters remaining after parsing.
// This may return non-empty slice if AllowCmdLineTrailingExtra
// or AllowCmdLineAllExtra are set to true in ConfigOptions, or if the
// command line contained double dash "--".
func (cfg *Config) RemainingArgs() []string {
	return cfg.remaining
}

// Returns the name of the processed configuration file or empty string
// if the configuration file was not processed.
func (cfg *Config) ProcessedConfigFile() string {
	return cfg.readFile
}

type valueType int
const (
	vt_invalid	valueType = iota
	vt_bool
	vt_string
	vt_int
	vt_uint
	vt_float
	vt_duration
	vt_struct
	vt_map
	vt_slice
)

type fieldInfo struct {
	fieldName		string
	names			[]string
	defaultValue	string
	vtype			valueType
	sevtype			valueType		// type of slice element, if primitive
	primitive		bool
	hasTags			bool
	configFile		bool
	exclude			bool
	mandatory		bool
	valueSource		ValueSource		
}

const (
	cfgfile_none 	 = 0			
    cfgfile_default  = 1
    cfgfile_explicit = 2
)    

// Parses the command line parameters and the configuration file,
// if configuration file was specified.
// Parameter "config" must be a pointer to the structure containing
// configuration fields. This function panics if the config parameter is not
// a non-nil poointer to some struct.
// All errors in the command line or configuration file are processed
// according to the CmdLineError and ConfigFileError values in ConfigOptions.
// By default both are set to ExitOnError.
func (options ConfigOptions) Parse(config interface{}) (*Config, error) {
	return parse(config, &options, true, true)
}
	
// Parses the command line parameters only.
// All options related to the configuration file parsing are ignored.
// Parameter "config" must be a pointer to the structure containing
// configuration fields. This function panics if the config parameter is not
// a non-nil poointer to some struct.
// All errors in the command line are processed according to the CmdLineError
// value in ConfigOptions which is set to ExitOnError by default.
func (options ConfigOptions) ParseCmdLine(config interface{}) (*Config, error) {
	return parse(config, &options, true, false)
}
	
// Parses the configuration file only.
// All options related to the command line parsing are ignored.
// Parameter "config" must be a pointer to the structure containing
// configuration fields. This function panics if the config parameter is not
// a non-nil poointer to some struct.
// All errors in the config file are processed according to the ConfigFileError
// value in ConfigOptions which is set to ExitOnError by default.
func (options ConfigOptions) ParseConfig(config interface{}) (*Config, error) {
	return parse(config, &options, false, true)
}
	
func parse(config interface{}, options *ConfigOptions,
	parseCmdLine, parseConfigFile bool) (*Config, error) {

	if config == nil {
		panic("config parameter is nil")
	}
	
	cfg := &Config{}
	cfg.options = (*options)
	cfg.parseCmdLine = parseCmdLine
	cfg.parseConfigFile = parseConfigFile
	
	err := cfg.parseImpl(config)
	
	if err == nil {
		return cfg, nil
	}
		
	if cfg.CmdLineError {
		switch cfg.options.CmdLineError {
			case ReturnError:
				return cfg, err;
			case ExitOnError:	
				fmt.Printf("Parameter error: %v\n", err)
				if cfg.haveHelp() && cfg.options.DisableHelpParams == false {
					fmt.Printf("Use -h or -help for help.\n")
				}				
				os.Exit(2)
			default:	
				panic(fmt.Sprintf("Parameter error: %v\n", err))
		}
	}
		
	if cfg.ConfigFileError {
		switch cfg.options.ConfigFileError {
			case ReturnError:
				return cfg, err;
			case ExitOnError:	
				fmt.Printf("Error reading configuration file '%s': %v\n",
					cfg.readFile, err)
				os.Exit(2)
			default:	
				panic(fmt.Sprintf("Error reading configuration file '%s': %v\n",
					cfg.readFile, err))
		} 
	}
	
	switch cfg.options.DataError {
		case ReturnError:
			return cfg, err;
		case ExitOnError:	
			fmt.Printf("Program error in configuration fields: %v\n",
				err)
			os.Exit(2)
		default:	
			panic(fmt.Sprintf("Program error in configuration fields: %v\n",
				err))
	} 

	return cfg, err			// for compiler
}

func (cfg *Config) parseImpl(config interface{}) error {

	var err error
	
	ct := reflect.ValueOf(config).Type()
	
	// root type, root kind, count of pointers
	rt, rk, ptrs := dereferenceType(ct)
	if ptrs != 1 || rk != reflect.Struct {
		panic(fmt.Errorf("config parameter must be a pointer to a " +
			"struct, passed parameter is of type '%v'", ct))	
	}
	
	cfg.config = reflect.ValueOf(config)
	cfg.fields = make(map[string]*fieldInfo)
	
	if cfg.options.Args == nil {
		cfg.args   = os.Args[1:]		// skip command at os.Args[0]
	} else {
		cfg.args   = cfg.options.Args
	}

	// Call inspect on the root type
	err = cfg.inspect(rt)
	if err != nil {
		return  err
	}
	
	// Parse command line
	if cfg.parseCmdLine {
		err = cfg.processCmdLine()
		if err != nil {
			cfg.CmdLineError = true
			return err
		}
		if cfg.helpRequested {
			cfg.printHelp()
			os.Exit(0)
		}
	}
	
	if cfg.parseConfigFile {
		// Parse config file. If user provided no default config
		// file name and did not define 'configfile' tag then we simply
		// do nothing.
		fileName, source := cfg.getConfigFileName()
		
		if len(fileName) > 0 {
			cfg.readFile = fileName
			err = cfg.readConfig(fileName, source)
			if err != nil {
				cfg.ConfigFileError = true
				return err 
			}			
		}		
	}
	
	// check mandatory params
	for _, fi := range cfg.fields {
		if fi.mandatory && (fi.valueSource == NotSet) {
			// mandatory parameter not specified anyhow
			// pick correct error type and name to report
			name := fi.fieldName
			if (cfg.parseConfigFile && (len(cfg.readFile) > 0)) ||
				cfg.parseCmdLine == false {
				cfg.ConfigFileError = true
			} else {
				cfg.CmdLineError = true
				if len(fi.names) > 0 {
					name = ""
					for _, n := range fi.names {
						if len(n) > len(name) {
							name = n
						}
					}
				}
				name = "-" + name						
			}
			return fmt.Errorf("mandatory parameter '%s' not specified", name)
		}
	}
			
	return nil		
}

func (cfg *Config) haveHelp() bool {
	return len(cfg.options.Help) > 0
}

func (cfg *Config) printHelp() {
	if len(cfg.options.Help) > 0 {
		fmt.Print(cfg.options.Help)
	}
	return
}

//===========================================================================
// Parse command line
//===========================================================================

func (cfg *Config) processCmdLine() error {

	cfg.canHaveExtra = cfg.options.AllowCmdLineTrailingExtra || 
					   cfg.options.AllowCmdLineAnyExtra 	

	for  {
		stop, err := cfg.parseOneCmd()
		if err != nil {
			cfg.CmdLineError = true
			cfg.remaining = append(cfg.remaining, cfg.args...)
			return err
		}
		if stop {
			break
		}
	}

	cfg.remaining = append(cfg.remaining, cfg.args...)

	if len(cfg.remaining) > 0 && 
		cfg.options.AllowCmdLineTrailingExtra == false &&
		cfg.options.AllowCmdLineAnyExtra == false {
		cfg.CmdLineError = true
		if len(cfg.remaining) == 1 {
			return fmt.Errorf("invalid parameter: %s", cfg.remaining[0])
		}
		return fmt.Errorf("invalid parameters: %v", cfg.remaining)
	}

	return nil
}

func (cfg *Config) parseOneCmd() (bool, error) {
	if len(cfg.args) == 0 {
		return true, nil
	}		

	src := cfg.args[0]
	if len(src) == 0 {
		// Ignore empty strings (???)
		return false, nil
	}
	
	if len(src) == 1 && src[0] == '-' {
		return false, fmt.Errorf("invalid parameter '%s'", src) 
	}
	
	if src[0] != '-' {
		if cfg.canHaveExtra {
			cfg.remaining = append(cfg.remaining, src)
			cfg.args = cfg.args[1:]
			return false, nil
		}
		return false, fmt.Errorf("invalid parameter '%s'", src) 
	}
	
	if len(cfg.remaining) > 0 {
		// We had extra params that we assumed are trailing, now they're not.
		if cfg.options.AllowCmdLineAnyExtra == false {
			cfg.args = append(cfg.remaining, cfg.args...)
			cfg.remaining = cfg.remaining[:0]
			return false, fmt.Errorf("invalid parameter '%s'", cfg.args[0]) 
		}
	}
	
	dash_num := 1
	if src[1] == '-' {
		dash_num++
		if len(src) == 2 {
			if cfg.options.AllowCmdLineDoubleDash {
				cfg.doubleDash = true
				cfg.args = cfg.args[1:]
				return true, nil
			}
			return false, fmt.Errorf("invalid parameter '%s'", src) 
		}
	}
	
	name := src[dash_num:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		return false, fmt.Errorf("invalid parameter syntax: %s", src) 
	}
	
	var strval string
	var eqval bool
	
	n := strings.Index(name, "=")
	if n == 0 {
		return false, fmt.Errorf("invalid parameter syntax: %s", src) 
	} else if n > 0{
		strval = name[n+1:]
		name = name[:n]
		eqval = true
	}
	
	if cfg.options.DisableHelpParams == false {
		if name == "h" || name == "help" {
			if eqval {
				return false, fmt.Errorf("parameter %s must have no value") 
			}
			cfg.args = cfg.args[1:]
			cfg.helpRequested = true
			return true, nil
		}
	}
	
	// Find struct field for this parameter
	fi := cfg.getField(name, false)

	if fi == nil {
		return false, fmt.Errorf("invalid parameter '%s'", src) 
	}
	
	cmdName := cfg.isCmdName(fi, name)
	if cfg.options.AllowAllCmdLine == false && cmdName == false {
		return false, fmt.Errorf("'%s' is not a valid parameter", src) 
	}
	
	sliceParam := false
	
	if fi.vtype == vt_slice && fi.sevtype != vt_invalid {
		sliceParam = true		// OK, allow multiple
	} else if fi.primitive {
		// OK as a cmd line param						
	} else {
		return false, fmt.Errorf("'%s' is not a valid parameter", src) 
	}

	// Check valid name
	if sliceParam == false && fi.valueSource == FromCmdLine {
		return false, fmt.Errorf("'%s' is a duplicate parameter", src) 
	}
	
	// We have a valid parameter
	
	// If all is OK only then remove the param (???)
	cfg.args = cfg.args[1:]
	
	// Get field value in the struct
	fieldVal := reflect.Indirect(cfg.config).FieldByName(fi.fieldName)
	
	if fi.vtype == vt_bool {
		if eqval == false {
			strval = "true"					 
		}
	} else {
		// All other should have a value
		if eqval == false {
			if len(cfg.args) < 1 {
				return false, fmt.Errorf("parameter '%s' must specify a value",
					src) 
			}
			strval = cfg.args[0]
			cfg.args = cfg.args[1:]
		}
	}
	
	errstr := cfg.setFromString(fi, fieldVal, strval)
	if len(errstr) > 0 {
		return false, fmt.Errorf("parameter '%s': %s", src, errstr)
	}
	fi.valueSource = FromCmdLine
	
	if fi.configFile {
		cfg.configFile = strval
	}
	
	return false, nil
}

//===========================================================================
// readConfig
//
// User may simply not define 'configfile' tag and provide no default
// config file name if there is no config to read, or call ParseCmdLine.
//===========================================================================

func (cfg *Config) getConfigFileName() (string, int) {
	fileName := cfg.configFile
	if len(fileName) > 0 {
		return fileName, cfgfile_explicit
	}
	fileName = cfg.options.DefaultConfigFile
	if len(fileName) > 0 {
		return fileName, cfgfile_default
	}
	return "", cfgfile_none
}


func (cfg *Config) readConfig(fileName string, source int) error {

	fileInfo, err := os.Lstat(fileName)
	if err != nil {
		if source == cfgfile_default && 
			cfg.options.ConfigFileMustExist == false {
			return nil
		}
		return err
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("specified path is a directory")
	}

	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("unable to open file: %s",
			err.(*os.PathError).Err.Error())
	}
	defer file.Close()

	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("unable to read file: %s", err.Error())
	}

	var m map[string]interface{}

	text := string(buf)
	err = json.Unmarshal([]byte(text), &m)
	if err != nil {
		return err
	}
	
	for name, value := range m {
		fi := cfg.getField(name, false)
		if fi == nil {
			return fmt.Errorf("JSON field '%s' is not a parameter", name)
		}
		if fi.valueSource == FromCmdLine {
			continue
		}
		bytes, err := json.Marshal(value)
		if err != nil {
			// not expected
			return fmt.Errorf("unable to marshal JSON field '%s'", name)
		}
		
		vs := reflect.Indirect(cfg.config)
		fv := vs.FieldByName(fi.fieldName)
	
		if fv.IsValid() == false || fv.CanSet() == false {
			return fmt.Errorf("unable to set field '%s' from config", name)
		}
	
		var sv interface{}
		if fv.CanAddr() {
			sv = fv.Addr().Interface()
		} else {
			sv = fv.Interface()
		}
		err = json.Unmarshal(bytes, sv)
		if err != nil {
			return fmt.Errorf("unable to set field '%s' from config: %v",
				name, err)
		}
		fi.valueSource = FromConfigFile
	}
	
	return nil
}

//===========================================================================
// setFromString
//
// Set parameter value from string. Used when parsing command line and
// when settingparam's value to the default specified in field tag.
//===========================================================================

func createField(fieldVal reflect.Value) reflect.Value {
	if fieldVal.Type().Kind() == reflect.Ptr {
	 	if fieldVal.IsNil() {
			fieldVal.Set(reflect.New(fieldVal.Type().Elem()))
		}
		fieldVal = reflect.Indirect(fieldVal)
	}
	return fieldVal
}

func (cfg *Config) setFromString(fi *fieldInfo, fieldVal reflect.Value,
	strval string) string {

	if fi.vtype == vt_slice {
	 	if fieldVal.IsNil() {
			ns := reflect.MakeSlice(fieldVal.Type(), 0, 8)
			reflect.Copy(ns, fieldVal)
			fieldVal.Set(ns)
		}
		n := fieldVal.Len()
		if n == fieldVal.Cap() {
			ns := reflect.MakeSlice(fieldVal.Type(), n, 2 * n)
			reflect.Copy(ns, fieldVal)
			fieldVal.Set(ns)
		}
		fieldVal.SetLen(n + 1)
		return cfg.setPrimFromString(fi.sevtype, fieldVal.Index(n), strval)
	}
	
	fieldVal = createField(fieldVal)
	return cfg.setPrimFromString(fi.vtype, fieldVal, strval)
}
	
func (cfg *Config) setPrimFromString(vt valueType, fieldVal reflect.Value,
	strval string) string {
	
	// TODO: resolve what to do with empty string, not sure if it
	// has to be an error...
	
	if vt == vt_int {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		nocommas := removeCommas(strval)
		i64, err := strconv.ParseInt(nocommas, 0, 64)
		if err != nil { 
			ne := err.(*strconv.NumError)
			if ne.Err == strconv.ErrRange {				
				return fmt.Sprintf("value is too large: %s", strval)
			} 
			return fmt.Sprintf("invalid syntax of integer: '%s'", strval)
		}
		if reflect.Indirect(fieldVal).OverflowInt(i64) {
			return fmt.Sprintf("value is out of range: %s", strval)
		}
		fieldVal.SetInt(i64)
		return ""
	}
	
	if vt == vt_uint {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		nocommas := removeCommas(strval)
		u64, err := strconv.ParseUint(nocommas, 0, 64)
		if err != nil { 
			ne := err.(*strconv.NumError)
			if ne.Err == strconv.ErrRange {				
				return fmt.Sprintf("value is too large: %s", strval)
			} 
			return fmt.Sprintf("invalid syntax of unsigned integer: '%s'", strval)
		}
		if reflect.Indirect(fieldVal).OverflowUint(u64) {
			return fmt.Sprintf("value is out of range: %s", strval)
		}
		fieldVal.SetUint(u64)
		return ""
	}
	
	if vt == vt_float {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		f64, err := strconv.ParseFloat(strval, 64)
		if err != nil { 
			ne := err.(*strconv.NumError)
			if ne.Err == strconv.ErrRange {				
				return fmt.Sprintf("value is out of range: %s", strval)
			} 
			return fmt.Sprintf("invalid syntax of float value: '%s'", strval)
		}
		if reflect.Indirect(fieldVal).OverflowFloat(f64) {
			return fmt.Sprintf("value is out of range: %s", strval)
		}
		fieldVal.SetFloat(f64)
		return ""
	}
	
	if vt == vt_string {
		fieldVal.SetString(strval)
		return ""
	}
	
	if vt == vt_bool {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		v := strings.ToLower(strval) 
		if v == "true" || v == "t" || v == "1" {
			fieldVal.SetBool(true)
		} else if v == "false" || v == "f" || v == "0" {
			fieldVal.SetBool(false)
		} else {
			return fmt.Sprintf("'%s' is not a valid boolean value", strval) 
		}		
		return ""
	}
	
	if vt == vt_duration {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		duration, err := time.ParseDuration(strval)
		if err != nil {
			return fmt.Sprintf("invalid syntax of duration: '%s'", strval)
		}
		fieldVal.Set(reflect.ValueOf(duration))		
		return ""
	}
	
	return fmt.Sprintf("invalid field value type %v", fieldVal)
}

// Remove commas from number string, i.e. 1,000,000 returns 1000000.
// Returns original string if "s" is not a valid comma-separated integer.
func removeCommas(s string) string {
	src := s
	sign := ""
	if len(s) > 0 && s[0] == '-' {
		sign = "-"
		s = s[1:]		
	}
	if len(s) <= 3 || s[0] == '0' || s[0] == ',' {
		return src
	}
	for _, r := range s {
		if (r != ',') && (r < '0' || r > '9') {
			return src
		}
	}
	rs := s
	n := strings.LastIndex(rs, ",")
	if n < 0 {
		return src
	} 
	ns := ""
	for {
		if n != len(rs)-4 {
			return src
		}
		ns = rs[n+1:] + ns
		rs = rs[:n]
		n = strings.LastIndex(rs, ",")
		if len(rs) <= 3 {
			if n >= 0 {
				return src
			}
			return sign + rs + ns
		}
	}
	return src		// not used, for compiler who want s a return
}

//===========================================================================
// Inspect fields of user's config struct
//===========================================================================

func dereferenceType(t reflect.Type) (reflect.Type, reflect.Kind, int) {
	n := 0
	for t.Kind() == reflect.Ptr {
		n++
		t = t.Elem()
	}
	return t, t.Kind(), n
}

func (cfg *Config) getField(name string, nocase bool) *fieldInfo {
	if nocase == false {
		return cfg.fields[name]
	}
	ln := strings.ToLower(name)				
	for fn, fi := range cfg.fields {
		if strings.ToLower(fn) == ln {
			return fi
		}
	}
	return nil
}

func (cfg *Config) isCmdName(fi *fieldInfo, name string) bool {
	for _, fn := range fi.names {
		if fn == name {
			return true
		}
	}
	return false
}
	
func (cfg *Config) inspect(structType reflect.Type) error {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		err := cfg.processField(&field)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cfg *Config) processField(field *reflect.StructField) error {

	var fieldName string
	var err error

	// "rt" is field's root type.
	rt, _, ptrs := dereferenceType(field.Type)

	// fieldName is for error text only
	if field.Anonymous == false {
		fieldName = field.Name
	} else {
		fieldName = rt.PkgPath() + "." + rt.Name()
	}
	
	if ptrs > 1 {
		return fmt.Errorf("too many pointers in field '%s'", fieldName)
	}
	
	// Ignore some
	if len(field.PkgPath) > 0 {
		return nil		// ignore unexported
	}
	if rt.Kind() == reflect.Struct && rt.NumField() == 0 {
		return nil		// ignore structs with no fields
	}
	if field.Anonymous && (rt.Kind() != reflect.Struct) {
		return nil		// ignore anonymous fields that are not struct
	}
	switch rt.Kind() {
	case reflect.Invalid, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return nil
	}
	
	fi := &fieldInfo{names:make([]string,0,2)}

	// Parse field tags.
	err = cfg.parseTags(fieldName, field, fi)
	if err != nil {
		return err
	}
	
	// Check tag consistency
	if fi.mandatory && (len(fi.defaultValue) > 0) {
		return fmt.Errorf("mandatory field '%s' must not specify default value",
			fieldName)
	}

	// If FieldInfo says Exclude then don't verify anything else? ???
	if fi.exclude {
		return nil
	}
	
	if field.Anonymous == true {
		if rt.Kind() == reflect.Struct {
			err = cfg.inspect(rt)
			if err != nil {
				return err
			}
			return nil
		}
		return nil			
	} 
	
	fi.fieldName = field.Name
	
	if len(fi.names) == 0 {
		fi.names = append(fi.names, fieldName)	
	}

	err = cfg.inspectField(fieldName, field, fi)
	if err != nil {
		return err
	}
	
	if fi.configFile && (fi.vtype != vt_string) {
		return fmt.Errorf("invalid type of field '%s': configfile field " +
			"must be a string", fieldName)
	}

	// set default value
	if len(fi.defaultValue) > 0 {
		if fi.primitive == false {
			return fmt.Errorf("illegal to specify default value for " +
				"non-primitive field '%s'", fieldName)
		} 
		fieldVal := reflect.Indirect(cfg.config).FieldByName(fi.fieldName)
		errstr := cfg.setFromString(fi, fieldVal, fi.defaultValue)
		if len(errstr) > 0 {
			return fmt.Errorf("invalid default value for field '%s': %s",
				fieldName, errstr)
		}
		fi.valueSource = FromDefault 
	}

	for _, name := range fi.names {
		if _, ok := cfg.fields[name]; ok == true {
			return fmt.Errorf("field '%s' defines parameter name " +
				"'%s' that is already defined by another field",
				fieldName, name)
		
		}
		cfg.fields[name] = fi
		if len(field.Name) > 0 {
			cfg.fields[fi.fieldName] = fi
		}			
	}
	
	return nil
}

func (cfg *Config) inspectField(fieldName string, field *reflect.StructField,
	fi *fieldInfo) error {

	ft, fk, ptrs := dereferenceType(field.Type)
	
	if ptrs > 1 {
		return fmt.Errorf("invalid type of field %s: "+
				"pointer-to-pointer not supported", fieldName)
	}

	switch fk {
		case reflect.Interface, reflect.Complex64, reflect.Complex128,
			 reflect.Uintptr:
			return fmt.Errorf("invalid type of field %s: "+
				"type '%v' not supported", fieldName, field.Type)
	}

	vt := cfg.getPrimType(ft, fk)

	if vt != vt_invalid {
		fi.vtype = vt
		fi.primitive = true
		return nil
	}

	if fk == reflect.Slice {
		fi.vtype = vt_slice
		srt, srk, sptrs := dereferenceType(ft.Elem())
		if sptrs == 0 {
			vt = cfg.getPrimType(srt, srk)
			if vt != vt_invalid {
				fi.sevtype = vt
			}
		}			 
		return nil		
	}
	
	if fk == reflect.Struct {
		fi.vtype = vt_struct
		return nil		
	}
	
	if fk == reflect.Map {
		fi.vtype = vt_map
		return nil		
	}
	
	return fmt.Errorf("unsupported type '%v' of field %s",
		field.Type, fieldName)
}


func (cfg *Config) getPrimType(t reflect.Type, k reflect.Kind) valueType {

	// Check first because the kind is int64
	if t == reflect.TypeOf((*time.Duration)(nil)).Elem() {
		return vt_duration
	}
	
	switch k {
		case reflect.Int, reflect.Int8, reflect.Int16,
			 reflect.Int32, reflect.Int64:
			return vt_int
		case reflect.Uint, reflect.Uint8, reflect.Uint16,
			 reflect.Uint32, reflect.Uint64:
			return vt_uint
		case reflect.Float32, reflect.Float64:
			return vt_float
		case reflect.String:
			return vt_string
		case reflect.Bool:
			return vt_bool
	}

	return vt_invalid
}

//===========================================================================
// Tags
//===========================================================================

func (cfg *Config) parseTags(fieldName string, field *reflect.StructField,
	fi *fieldInfo) error {

	tag := field.Tag.Get(confTagsName)
	if tag == "" {
		return nil
	}
	
	fi.hasTags = true

	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return nil
	}

	tags := make(map[string]string) // to check for duplicates

	var src_name, name, value string

	for i := 0; i < len(parts); i++ {
		sv := parts[i]
		nv := strings.Split(sv, "=")
		num := len(nv)
		if num > 2 {
			return fmt.Errorf("invalid tags for %s: '%s' is invalid syntax.",
				fieldName, sv)
		}
		if num == 2 {
			src_name = nv[0]
			value = nv[1]
		} else {
			src_name = sv
			value = ""
		}
		name = strings.ToLower(src_name)
		if tags[name] != "" {
			return fmt.Errorf("invalid tags for %s: duplicate tag '%s'",
				fieldName, src_name)
		}
		tags[name] = name

		err := cfg.parseTag(fieldName, src_name, num, name, value, fi)
		if err != nil {
			return err
		}
	}

	return nil
}

// check tag has value (called for those that must)
func checkTagValue(fieldName, src_name string, num int, value string) error {
	if num != 2 || value == "" {
		return fmt.Errorf("invalid tag for field '%s': '%s' must have value",
			fieldName, src_name)
	}
	return nil
}

// struct name and src_name to use in errors.
func (cfg *Config) parseTag(fieldName, src_name string, num int,
	name, value string, fi *fieldInfo) error {

	var err error

	switch name {
		case "cmd":
			err = checkTagValue(fieldName, src_name, num, value)
			if err != nil {
				return err
			}
			a := strings.Split(value,"|")
			for _, n := range a {
				fi.names = append(fi.names, n)
			}
			return nil
	
		case "default":
			err = checkTagValue(fieldName, src_name, num, value)
			if err != nil {
				return err
			}
			fi.defaultValue = value
			return nil
	}

	switch name {
		case "configfile":
			if num > 1 {
				return fmt.Errorf("invalid tag for field %s: " +
					"'%s' must not have value.", fieldName, src_name)
			}
			if cfg.ncfgfile > 0 {
				return fmt.Errorf("invalid tag for field %s: " +
					"duplicate 'configfile' tag", fieldName)
			}
			cfg.ncfgfile++
			fi.configFile = true
			return nil
		case "must":
			if num > 1 {
				return fmt.Errorf("invalid tag for field %s: " +
					"'%s' must not have value.", fieldName, src_name)
			}
			fi.mandatory = true
			return nil
		case "-":
			if num > 1 {
				return fmt.Errorf("invalid tag for field %s: " +
					"'%s' must not have value.", fieldName, src_name)
			}
			fi.exclude = true
			return nil
	}

	return fmt.Errorf("invalid tag for field %s: '%s' is not a valid tag",
		fieldName, src_name)
}

