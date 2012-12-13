// Copyright 2012 Apcera Inc. All rights reserved.

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const cfgTagName = "cfg"

type SetType int
const (
	NotSet			SetType = iota
	SetFromDefault
	SetFromCommand
	SetFromConfig
)

type ErrorHandling int
const (
	// Requests atomatic exit on any error in command line parameters.
	ExitOnError ErrorHandling = iota
	// If command line errors are encountered the error is returned
	// programmatically from the Parse() function.
	ReturnError
	ContinueOnError
)

type dataType int
const (
	dt_invalid	dataType = iota
	dt_bool
	dt_string
	dt_int
	dt_uint
	dt_float
	dt_struct
	dt_map
	dt_slice
)

type fieldInfo struct {
	fieldName		string
	names			[]string
	defaultValue	string
	vtype			dataType
	hasTags			bool
	configFile		bool
	exclude			bool
	mandatory		bool
	setType			SetType
}

type ConfigOptions struct {

	// Specifies command line arguments to parse. If this is nil then
	// parameters are parsed from os.Args, as usual. This allows to
	// override using os.Args.
	Args				[]string

	// If not empty, specifies default config file name.
	DefaultConfigFile	string

	// Specifies reaction to errors in the command line parameters.  
	OnError				ErrorHandling

	// If true then parameters '-h' and '-help' are not automatically
	// processed as a help request.  
	DisableHelpParams	bool
	
	// If true then Parse() function does not parse the
	// command line parameters and pases only the config file.
	ConfigFileOnly		bool
	
	// If true and the default config file name is specified then
	// it must exist. By default only cofig file excplicitly specified
	// on the command line must exist.
	ConfigMustExist		bool
	
	// Help printed into console in case of errors in the
	// command line parameters.
	Help				string
}

type Config struct {

	options			ConfigOptions
	args			[]string				// from user or os.Args from 1
	help			string
	
	configFile		string
	ncfgfile		int
	readFile		string					// what we actually read if any

	fields			map[string]*fieldInfo
	cmdMap 			map[string]interface{}	// from cmd line
	cfgMap			map[string]interface{}	// from config file
	
	configFileError	bool
	cmdLineError	bool
	helpRequested	bool
	
	config			reflect.Value
}

const (
	cfgfile_none 	 = 0			
    cfgfile_default  = 1
    cfgfile_explicit = 2
)    

func (options ConfigOptions) Parse(config interface{}) (*Config, error) {
	cfg, err := (&options).parse(config)
	
	if err != nil {
		
		if cfg == nil {
			return nil, err
		}
		
		if cfg.configFileError {
			fmt.Printf("Error reading configuration file '%s': %v\n",
				cfg.readFile, err)
			os.Exit(1)
		}
		
		if options.OnError == ExitOnError {
			fmt.Printf("Parameter error: %v\n", err)
			if cfg.cmdLineError {
				cfg.printHelp()
			}
			os.Exit(1)
		}
		return nil, err
	}
	
	return cfg, nil
}

func (options *ConfigOptions) parse(config interface{}) (*Config, error) {

	if config == nil {
		panic("config structure is nil")
	}
	
	var err error
	
	ct := reflect.ValueOf(config).Type()
	
	// root type, root kind, count of pointers
	rt, rk, ptrs := dereferenceType(ct)
	if ptrs != 1 || rk != reflect.Struct {
		return nil, fmt.Errorf("config parameter must be a pointer to a " +
			"struct, passed parameter is of type '%v'", ct)	
	}
	
	cfg := &Config{}
	cfg.options = (*options)
	cfg.cmdMap = make(map[string]interface{})
	cfg.cfgMap = make(map[string]interface{})
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
		return cfg, err
	}
	
	// Parse command line
	if cfg.options.ConfigFileOnly == false {
		err = cfg.parseCmdLine()
		if err != nil {
			cfg.cmdLineError = true
			return cfg, err
		}
		if cfg.helpRequested {
			cfg.printHelp()
			os.Exit(0)
		}
	}
	
	// Parse config file. If user provided no default config
	// file name and did not define 'configfile' tag then we simply
	// do nothing.
	fileName, source := cfg.getConfigFileName()
	
	if len(fileName) > 0 {
		cfg.readFile = fileName
		err = cfg.readConfig(fileName, source)
		if err != nil {
			cfg.configFileError = true
			return cfg, err 
		}			
	}		
		
	return cfg, nil		
}

func (cfg *Config) printHelp() {
	if len(cfg.options.Help) > 0 {
		fmt.Print(cfg.options.Help)
		return
	}
	if len(cfg.help) > 0 {
		fmt.Print(cfg.help)
	}
}

//===========================================================================
// Parse command line functions
//===========================================================================
func (cfg *Config) parseCmdLine() error {
	for  {
		end, err := cfg.parseOneCmd()
		if err != nil {
			return err
		}
		if end {
			break
		}
	}
	return nil
}

func (cfg *Config) parseOneCmd() (bool, error) {
	if len(cfg.args) == 0 {
		return true, nil
	}		

	s := cfg.args[0]
	if len(s) <= 1 || s[0] != '-' {
		return true, nil
	}
	
	num_minuses := 1
	if s[1] == '-' {
		num_minuses++
		if len(s) == 2 {
			cfg.args = cfg.args[1:]
			return true, nil
		}
	}
	
	name := s[num_minuses:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		return false, fmt.Errorf("invalid parameter syntax: %s", s) 
	}
	
	if cfg.options.DisableHelpParams == false {
		if name == "h" || name == "help" {
			cfg.helpRequested = true
			return true, nil
		}
	} 
	
	// Find struct field for this
	fi := cfg.getField(name, false)
	if fi == nil {

		// try splitting them
		/*
		splitName = name
		for {
			if len(splitName) == 0 {
				break
			}
			splitPart = splitName[:1]
			splitName = splitName[1:]
			fi := cfg.getField(splitPart, false)
			if fi == nil {
				break
			}
			if fi.vtype == dt_bool {
				// a switch has no parameters and we do allow
				// to repeat it because it has no effect
				fieldVal.SetBool(true)
				fi.setType = SetFromCommand
				cfg.args = cfg.args[1:]
				return false, nil		
			}
		}
		*/		

		if cfg.options.OnError == ContinueOnError {
			// We hit something we don't understand. Then if user asked to
			// continue we keep all rest of params and abort parsing the
			// command line
			return true, nil
		}
		return false, fmt.Errorf("'%s' is not a valid parameter", s) 
	}
	
	// Get field value in the struct
	fieldVal := reflect.Indirect(cfg.config).FieldByName(fi.fieldName)

	if fi.vtype == dt_bool {
		// a switch has no parameters and we do allow to repeat it
		// because it has no effect
		fieldVal.SetBool(true)
		fi.setType = SetFromCommand
		cfg.args = cfg.args[1:]
		return false, nil		
	}
	
	if fi.setType == SetFromCommand {
		return false, fmt.Errorf("'%s' is a duplicate parameter", s) 
	}
	
	// All other should have a value
	if len(cfg.args) < 2 {
		return false, fmt.Errorf("parameter '%s' must have a value", s) 
	}
	
	strval := cfg.args[1]
	cfg.args = cfg.args[2:]
	
	errstr := cfg.setFromString(fi, fieldVal, strval)
	if len(errstr) > 0 {
		return false, fmt.Errorf("parameter '%s': %s", s, errstr)
	}
	fi.setType = SetFromCommand
	
	if fi.configFile {
		cfg.configFile = strval
	}
	
	return false, nil
}

//===========================================================================
// reaqConfig
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
		if source == cfgfile_default && cfg.options.ConfigMustExist == false {
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
		if fi.setType == SetFromCommand {
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
		fi.setType = SetFromConfig
	}
	
	return nil
}

//===========================================================================
// setFromString
//
// Set parameter value from string. Used when parsing command line and
// when settingparam's value to the default specified in field tag.
//===========================================================================
func (cfg *Config) setFromString(fi *fieldInfo, fieldVal reflect.Value,
	strval string) string {
	
	if fi.vtype == dt_int {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		i64, err := strconv.ParseInt(strval, 10, 64)
		if err != nil { 
			ne := err.(*strconv.NumError)
			if ne.Err == strconv.ErrRange {				
				return fmt.Sprintf("value is too large: %s", strval)
			} 
			return fmt.Sprintf("invalid syntax of integer: '%s'", strval)
		}
		if fieldVal.OverflowInt(i64) {
			return fmt.Sprintf("value is out of range: %s", strval)
		}
		fieldVal.SetInt(i64)
		return ""
	}
	
	if fi.vtype == dt_uint {
		if len(strval) == 0 {
			return fmt.Sprintf("value must not be an empty string") 
		}
		u64, err := strconv.ParseUint(strval, 10, 64)
		if err != nil { 
			ne := err.(*strconv.NumError)
			if ne.Err == strconv.ErrRange {				
				return fmt.Sprintf("value is too large: %s", strval)
			} 
			return fmt.Sprintf("invalid syntax of unsigned integer: '%s'", strval)
		}
		if fieldVal.OverflowUint(u64) {
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
		if fieldVal.OverflowFloat(f64) {
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

	// Because of case or no case we iterate over all fields.
	// Not too good but will do for the config parsing code.
	ln := name
	if nocase {
		ln = strings.ToLower(name)				
	}
	
	for fn, fi := range cfg.fields {
		if nocase {
			if strings.ToLower(fn) == ln {
				return fi
			}
		} else {
			if fn == ln {
				return fi
			}
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
		return fmt.Errorf("invalid type of field '%s': config file field " +
			"must be a string", fieldName)
	}

	//if field.Name == "Help" && fi.vtype == dt_string {
	//	fieldVal := reflect.Indirect(cfg.config).FieldByName(fi.fieldName)
	//	cfg.help = fieldVal.Interface().(string)
	//	return nil
	//}

	// set default value
	if len(fi.defaultValue) > 0 {
		fieldVal := reflect.Indirect(cfg.config).FieldByName(fi.fieldName)
		errstr := cfg.setFromString(fi, fieldVal, fi.defaultValue)
		if len(errstr) > 0 {
			return fmt.Errorf("invalid default value for field '%s': %s",
				fieldName, errstr)
		}
		fi.setType = SetFromDefault 
	}

	for _, name := range fi.names {
		if _, ok := cfg.fields[name]; ok == true {
			return fmt.Errorf("field '%s' defines parameter name " +
				"'%s' that is already defined by another field",
				fieldName, name)
		
		}
		cfg.fields[name] = fi
	}
	
	return nil
}

func (cfg *Config) inspectField(fieldName string, field *reflect.StructField,
	fi *fieldInfo) error {

	//cfgval := reflect.ValueOf(cfg.config)
	
	//ft, fk, ptrs := dereferenceType(field.Type)
	_, fk, ptrs := dereferenceType(field.Type)
	
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

	switch fk {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fi.vtype = dt_int
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fi.vtype = dt_uint
		return nil
	case reflect.Float32, reflect.Float64:
		fi.vtype = dt_float
		return nil
	case reflect.String:
		fi.vtype = dt_string
		return nil
	case reflect.Bool:
		fi.vtype = dt_bool
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
			return fmt.Errorf("invalid tag for %s: '%s' is invalid syntax.",
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
			return fmt.Errorf("invalid tag for %s: duplicate '%s'",
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
	case "name":
		if err = checkTagValue(fieldName, src_name, num, value); err != nil {
			return err
		}
		a := strings.Split(value,":")
		for _, n := range a {
			fi.names = append(fi.names, n)
		}
		return nil

	case "default":
		if err = checkTagValue(fieldName, src_name, num, value); err != nil {
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
	// ???
	case "-":
		if num > 1 {
			return fmt.Errorf("invalid tag for field %s: " +
				"'%s' must not have value.", fieldName, src_name)
		}
		fi.exclude = true
		return nil
	case "must":
		if num > 1 {
			return fmt.Errorf("invalid tag for field %s: " +
				"'%s' must not have value.", fieldName, src_name)
		}
		fi.mandatory = true
		return nil
	}

	return fmt.Errorf("invalid tag for field %s: '%s' is not a valid tag",
		fieldName, src_name)
}

