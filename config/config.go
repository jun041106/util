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
		// or in the configuration file.
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

	Anonymous struct fields are allowed and treated as if their fields are
	included directly into the top-level struct.
		
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

const cfgTagName = "conf"

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
// parameters.
type ErrorHandling int
const (
	// Print error message and exit on any error.
	ExitOnError ErrorHandling = iota
	
	// If error is encountered it is returned programmatically by the
	// Parse...() functions.
	ReturnError
	
	// Stop parsing command line on first error and leave invalid parameter
	// in remaining parameters. Errors in the configuration file are ignored.
	ContinueOnError
)

type ConfigOptions struct {

	// Specifies default config file name. May remain empty if no
	// default config file should be parsed.
	DefaultConfigFile	string

	// If true and the default config file name is specified then
	// it must exist. By default only config file explicitly specified
	// on the command line must exist.
	ConfigFileMustExist	bool
	
	// Specifies reaction to errors in the command line parameters.  
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
	
	// If true then all extra non-parameter values are ignored and
	// collected in remaining params. By default the command line can
	// contain only valid parameters and their values, presence of any
	// extra values is an error.
	AllowExtraCmdLine		bool
	
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

	options			ConfigOptions			// copy of user's options
	args			[]string				// from user or os.Args from 1
	remaining		[]string
	
	parseCmdLine	bool					// need to parse cmd line
	parseConfigFile	bool					// parse config file if specified
	
	configFile		string					// value from cmd line					
	ncfgfile		int						// track configfile tags
	readFile		string					// what we actually read, if any

	fields			map[string]*fieldInfo	// mapped by each name
	
	cmdLineError	bool					// error was in cmd line
	configFileError	bool					// error was in config file

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
func (cfg *Config) RemainingArgs() []string {
	return cfg.remaining
}

type dataType int
const (
	dt_invalid	dataType = iota
	dt_bool
	dt_string
	dt_int
	dt_uint
	dt_float
	dt_duration
	dt_struct
	dt_map
	dt_slice
)

type fieldInfo struct {
	fieldName		string
	names			[]string
	defaultValue	string
	vtype			dataType
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
// if configuration file was specified. Parameter "config" must be a pointer
// to the structure containing configuration fields.
// Currently this function exits on any error, error is never returned.
func (options ConfigOptions) Parse(config interface{}) (*Config, error) {
	cfg := &Config{}
	cfg.options = options
	cfg.parseCmdLine = true
	cfg.parseConfigFile = true
	return cfg.parse(config)
}
	
// Parses the command line parameters only.
// All options related to the configuration file parsing are ignored.
// Parameter "config" must be a pointer to the structure containing
// configuration fields.
// Currently this function exits on any error, error is never returned.
func (options ConfigOptions) ParseCmdLine(config interface{}) (*Config, error) {
	cfg := &Config{}
	cfg.options = options
	cfg.parseCmdLine = true
	return cfg.parse(config)
}
	
// Parses the configuration file only.
// All options related to the command line parsing are ignored.
// Parameter "config" must be a pointer to the structure containing
// configuration fields.
// Currently this function exits on any error, error is never returned.
func (options ConfigOptions) ParseConfig(config interface{}) (*Config, error) {
	cfg := &Config{}
	cfg.options = options
	cfg.parseConfigFile = true
	return cfg.parse(config)
}
	
func (cfg *Config) parse(config interface{}) (*Config, error) {

	err := cfg.parseImpl(config)
	
	if err != nil {
		
		if cfg == nil {
			// temp
			fmt.Printf("%v\n", err)
			os.Exit(2)			
			// TODO: finish
			//return nil, err
		}
		
		if cfg.cmdLineError {
			// TODO: finish. currently always exit
			//if cfg.options.CmdLineError == ExitOnError {
				fmt.Printf("Parameter error: %v\n", err)
				if cfg.haveHelp() && cfg.options.DisableHelpParams == false {
					fmt.Printf("Use -h or -help for help.\n")
				}				
				os.Exit(2)
			//}				

			// TODO: finish
			//return cfg, err
		}
		
		if cfg.configFileError {
			fmt.Printf("Error reading configuration file '%s': %v\n",
				cfg.readFile, err)
			os.Exit(2)
		}
		
		// Otherwise it is a programmatic error
		fmt.Printf("%v\n", err)
		os.Exit(2)			
		//return nil, err
	}
	
	return cfg, nil
}

func (cfg *Config) parseImpl(config interface{}) error {

	if config == nil {
		panic("config structure is nil")
	}
	
	var err error
	
	ct := reflect.ValueOf(config).Type()
	
	// root type, root kind, count of pointers
	rt, rk, ptrs := dereferenceType(ct)
	if ptrs != 1 || rk != reflect.Struct {
		return fmt.Errorf("config parameter must be a pointer to a " +
			"struct, passed parameter is of type '%v'", ct)	
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
			cfg.cmdLineError = true
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
				cfg.configFileError = true
				return err 
			}			
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
// Parse command line functions
//===========================================================================
func (cfg *Config) processCmdLine() error {
	for  {
		stop, err := cfg.parseOneCmd()
		if err != nil {
			cfg.cmdLineError = true
			cfg.remaining = append(cfg.remaining, cfg.args...)
			return err
		}
		if stop {
			break
		}
	}
	cfg.remaining = append(cfg.remaining, cfg.args...)
	if len(cfg.remaining) > 0 && cfg.options.AllowExtraCmdLine == false {
		cfg.cmdLineError = true
		return fmt.Errorf("invalid parameters: %v", cfg.remaining) 
	}
	return nil
}

func (cfg *Config) parseOneCmd() (bool, error) {
	if len(cfg.args) == 0 {
		return true, nil
	}		

	src := cfg.args[0]
	if len(src) <= 1 || src[0] != '-' {
		if cfg.options.AllowExtraCmdLine {
			cfg.remaining = append(cfg.remaining, src)
			cfg.args = cfg.args[1:]
			return false, nil
		}
		return true, nil
	}
	
	dash_num := 1
	if src[1] == '-' {
		dash_num++
		if len(src) == 2 {
			cfg.doubleDash = true
			cfg.remaining = append(cfg.remaining, cfg.args[1:]...)
			cfg.args = make([]string, 0, 1)
			return true, nil
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
	
	cfg.args = cfg.args[1:]
	
	if cfg.options.DisableHelpParams == false {
		if name == "h" || name == "help" {
			if eqval {
				return false, fmt.Errorf("parameter %s must have no value") 
			}
			cfg.helpRequested = true
			return true, nil
		}
	} 
	
	// Find struct field for this
	fi := cfg.getField(name, false)
	if fi == nil {
		if cfg.options.CmdLineError == ContinueOnError {
			// We hit something we don't understand. Then if user asked to
			// continue we keep all rest of params and abort parsing the
			// command line
			return true, nil
		}
		return false, fmt.Errorf("'%s' is not a valid parameter", src) 
	}
	
	// Check valid name
	if cfg.options.AllowAllCmdLine == false {
		found := false
		if len(fi.names) > 0 {
			for _, fn := range fi.names {
				if fn == name {
					found = true
					break
				}
			}
		}
		if found == false {
			return false, fmt.Errorf("'%s' is not a valid parameter", src) 
		}			
	}
	
	// Get field value in the struct
	fieldVal := reflect.Indirect(cfg.config).FieldByName(fi.fieldName)
	
	if fi.valueSource == FromCmdLine {
		return false, fmt.Errorf("'%s' is a duplicate parameter", src) 
	}
	
	if fi.vtype == dt_bool {
		// a switch has no parameters and we do allow to repeat it
		// because it has no effect
		fi.valueSource = FromCmdLine
		bv := "true"
		if eqval {
			if strval == "true" || strval == "t" || strval == "True" ||
				strval == "TRUE" || strval == "1" {
				bv = "true"
			} else if strval == "false" || strval == "f" || strval == "False" ||
				strval == "FALSE" || strval == "0" {
				bv = "false"
			} else {
				return false, fmt.Errorf("'%s' is not a valid boolean " +
					"value of parameter '%s'", strval, name) 
			}	 
		}
		errstr := cfg.setFromString(fi, fieldVal, bv)
		if len(errstr) > 0 {
			return false, fmt.Errorf("error setting boolean " +
				"value of parameter '%s': %v", name, errstr) 
		}
		return false, nil		
	}
	
	// All other should have a value
	if eqval == false {
		if len(cfg.args) < 1 {
			return false, fmt.Errorf("parameter '%s' must have a value", src) 
		}
		
		strval = cfg.args[0]
		cfg.args = cfg.args[1:]
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
// config file name if there is no config to read.
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
	
	// TODO: resolve what to do with empty string, not sure if it
	// has to be an error...
	
	fieldVal = createField(fieldVal)

	if fi.vtype == dt_int {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		i64, err := strconv.ParseInt(strval, 0, 64)
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
	
	if fi.vtype == dt_uint {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		u64, err := strconv.ParseUint(strval, 0, 64)
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
	
	if fi.vtype == dt_float {
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
	
	if fi.vtype == dt_string {
		fieldVal.SetString(strval)
		return ""
	}
	
	if fi.vtype == dt_bool {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		if strings.ToLower(strval) == "true" {
			fieldVal.SetBool(true)
		} else if strings.ToLower(strval) == "false" {
			fieldVal.SetBool(false)
		} else {
			return fmt.Sprintf("not a boolean value: '%s'", strval)
		}		
		return ""
	}
	
	if fi.vtype == dt_duration {
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
	
	panic("not implemented here")
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
	if field.Anonymous && (rt.Kind() != reflect.Struct) {
		return nil
	}
	switch rt.Kind() {
	case reflect.Invalid, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return nil
	}
	if rt.Kind() == reflect.Struct && rt.NumField() == 0 {
		// ignore structs with no fields
		return nil
	}
	if len(field.PkgPath) > 0 {
		// ignore unexported
		return nil
	}
	
	fi := &fieldInfo{names:make([]string,0,2)}

	// Parse field tags.
	err = cfg.parseTags(fieldName, field, fi)
	if err != nil {
		return err
	}

	// If FieldInfo says Exclude then don't verify anything else?
	// ???
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

	if fi.configFile && (fi.vtype != dt_string) {
		return fmt.Errorf("invalid type of field '%s': configfile field " +
			"must be a string", fieldName)
	}

	// set default value
	if len(fi.defaultValue) > 0 {
		if fi.primitive == false {
			// TODO: allow time.Duration
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

	//cfgval := reflect.ValueOf(cfg.config)
	
	//ft, fk, ptrs := dereferenceType(field.Type)
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

	// Check first because the kind is int64
	if ft == reflect.TypeOf((*time.Duration)(nil)).Elem() {
		fi.vtype = dt_duration
		return nil		
	}
	
	switch fk {
		case reflect.Int, reflect.Int8, reflect.Int16,
			 reflect.Int32, reflect.Int64:
			fi.vtype = dt_int
			fi.primitive = true
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16,
			 reflect.Uint32, reflect.Uint64:
			fi.vtype = dt_uint
			fi.primitive = true
			return nil
		case reflect.Float32, reflect.Float64:
			fi.vtype = dt_float
			fi.primitive = true
			return nil
		case reflect.String:
			fi.vtype = dt_string
			fi.primitive = true
			return nil
		case reflect.Bool:
			fi.vtype = dt_bool
			fi.primitive = true
			return nil
	}

	if fk == reflect.Struct {
		fi.vtype = dt_struct
		return nil		
	}
	
	if fk == reflect.Slice {
		fi.vtype = dt_slice
		return nil		
	}
	
	if fk == reflect.Map {
		fi.vtype = dt_map
		return nil		
	}
	
	return fmt.Errorf("unsupported type '%v' of field %s",
		field.Type, fieldName)
}


//===========================================================================
// Tags
//===========================================================================

func (cfg *Config) parseTags(fieldName string, field *reflect.StructField,
	fi *fieldInfo) error {

	tag := field.Tag.Get(cfgTagName)
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
