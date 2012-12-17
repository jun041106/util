// Copyright 2012 Apcera Inc. All rights reserved.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type commonConfig struct {
	URL		string
	Port	int32		`conf:"cmd=p|port,default=4222"`
}

type dbConfig struct {
	URL			string
	Database	string
	Username	string
}

type testConfig struct {
	commonConfig
	Debug	bool	`conf:"cmd=d|debug"`
	Trace	bool	`conf:"cmd=t|trace"`
	Verbose	bool	`conf:"cmd=v|verbose"`
	Level	int		`conf:"default=99"`
	PInt	*int	`conf:"cmd=z"`
	Name	string	`conf:"default=apcera"`
	Db		*dbConfig
}

var help =
` Usage: command [-d]
  
  -p | -port   Listen port
  -d           Debug mode
`

var cmdLineSample = []string {
	"-d",
	"-p", "1234",
	"-z=3333",
	"-Level", "7777",
}

var config_text =
`
{
	"Level"	: 987654,
	"Db"	: { "URL" : "mysql://host:port", "foo" : 123 }
}
`

var _print = false

func writeFile(fileName string, content string) error {
	return ioutil.WriteFile(fileName, []byte(content), 0666)
}

func TestCore(t *testing.T) {
	fileName := filepath.Join(os.TempDir(),
		fmt.Sprintf("apcera_config_test.%d", time.Now().UnixNano()))

	os.Remove(fileName)
	defer os.Remove(fileName)
	err := writeFile(fileName, config_text)		
	if err != nil {
		t.Fatalf("***Error*** returned from Parse: %v\n", err)
	}

	var cfg *Config

	opts := &ConfigOptions{Help:help,DefaultConfigFile:fileName}
	opts.Args = cmdLineSample
	config := &testConfig{}
	cfg, err = opts.Parse(config)
	if err != nil {
		t.Fatalf("***Error*** returned from Parse: %v\n", err)
	}
	if _print {
		fmt.Printf("----------------------------------------------\n")
		fmt.Printf("Config after parsed:\n%+v\n", config)
		fmt.Printf("  PInt: %+v\n", *config.PInt)
		fmt.Printf("  Db:   %+v\n", config.Db)
		fmt.Printf("----------------------------------------------\n")
		fmt.Printf("Source of Level is %v\n", cfg.GetValueSource("Level"))
		fmt.Printf("Source of Port  is %v\n", cfg.GetValueSource("Port"))
	}		
}

//===========================================================================
// Test command-line data types, values and syntax
//===========================================================================

type testTypesConfig struct {
	T   	bool			`conf:"cmd=t"`
	F		bool			`conf:"cmd=f"`
	S		string			`conf:"cmd=s"`
	I		int   			`conf:"cmd=i"`
	I8		int8  			`conf:"cmd=i8"`
	I16		int16 			`conf:"cmd=i16"`
	I32		int32 			`conf:"cmd=i32"`
	I64		int64 			`conf:"cmd=i64,default=8761"`
	U		uint  			`conf:"cmd=u"`
	U8		uint8 			`conf:"cmd=u8,default=44"`
	U16		uint16			`conf:"cmd=u16"`
	U32		uint32			`conf:"cmd=u32"`
	U64		uint64			`conf:"cmd=u64"`
	F32		float32			`conf:"cmd=f32"`
	F64		float64			`conf:"cmd=f64"`
	D		time.Duration	`conf:"cmd=d,default=2000ms"`
	// We allow any to be a pointer. Rare use but is supported.
	PT		*bool			`conf:"cmd=pt"`		
	PF		*bool			`conf:"cmd=pf"`		
	PS		*string			`conf:"cmd=ps"`		
	PI		*int   			`conf:"cmd=pi"`
	PI8		*int8  			`conf:"cmd=pi8,default=126"`
	PI16	*int16 			`conf:"cmd=pi16"`
	PI32	*int32 			`conf:"cmd=pi32,default=-666"`
	PI64	*int64 			`conf:"cmd=pi64"`
	PU		*uint  			`conf:"cmd=pu"`
	PU8		*uint8 			`conf:"cmd=pu8"`
	PU16	*uint16			`conf:"cmd=pu16"`
	PU32	*uint32			`conf:"cmd=pu32"`
	PU64	*uint64			`conf:"cmd=pu64,default=77"`
	PF32	*float32		`conf:"cmd=pf32"`
	PF64	*float64		`conf:"cmd=pf64,default=2012.2012"`
	PD		*time.Duration	`conf:"cmd=pd"`
}

var cmdTypes = []string {
	"-t", "--f=false", "-s", "string",
	"-i", "0x10", "--i8", "-11", "-i16=12",
	"--i32=-1000000000", "--i64=10,000,000,000",
	"-u", "30", "--u8", "0x31", "-u16=0x7fff",
	"--u32=2000000000", "--u64", "20000000000",
	"-f32=123.123", "--f64", "4444.5555",
	"-d", "200ms",
	"-pt=1", "--pf=False", "-ps", "pstring2",
	"-pi", "0x20", "--pi8", "-11", "-pi16=12",
	"--pi32=-1,000,000,030", "--pi64=10000000030",
	"-pu", "33", "--pu8", "0x31", "-pu16=0x7ff1",
	"--pu32=2000000001", "--pu64", "20000000001",
	"-pf32=777.888", "--pf64", "4321.1234",
	"-pd=1h",
}

func TestCmdLineTypes(t *testing.T) {
	var pt = true
	var pf = false
	var ps = "pstring2"
	var pi   int 	= 0x20
	var pi8  int8 	= -11
	var pi16 int16	= 12
	var pi32 int32	= -1000000030
	var pi64 int64	= 10000000030
	var pu   uint	= 33 
	var pu8  uint8	= 0x31
	var pu16 uint16	= 0x7ff1
	var pu32 uint32	= 2000000001
	var pu64 uint64	= 20000000001
	var pf32 float32= 777.888
	var pf64 float64= 4321.1234
	var pd   time.Duration = 1 * time.Hour
	
	model := &testTypesConfig{T:true,F:false,S:"string",
		I:16, I8:-11, I16:12, I32:-1000000000, I64:10000000000,
		U:30, U8:49, U16:32767, U32:2000000000, U64:20000000000,
		F32:123.123, F64:4444.5555, D:(200*time.Millisecond), 
		}

	model.PT 	= &pt	
	model.PF 	= &pf	
	model.PS 	= &ps	
	model.PI 	= &pi
	model.PI8 	= &pi8
	model.PI16 	= &pi16
	model.PI32 	= &pi32
	model.PI64 	= &pi64
	model.PU   	= &pu
	model.PU8  	= &pu8
	model.PU16 	= &pu16
	model.PU32 	= &pu32
	model.PU64 	= &pu64
	model.PF32 	= &pf32
	model.PF64 	= &pf64
	model.PD 	= &pd
	
	opts := &ConfigOptions{Args:cmdTypes}
	config := &testTypesConfig{}
	config.F = true

	// test we preserve pointers that are not nil
	var ti int = 8888
	var tu32 uint32 = 99999
	config.PI = &ti
	config.PU32 = &tu32

	_, err := opts.Parse(config)
	if err != nil {
		t.Fatalf("***Error*** %v\n", err)
	}
	testeq(t, config, model)
	testnn(t, int64(ti), 0x20)
	testnn(t, int64(tu32), 2000000001)
}

//===========================================================================
// Test extras
//===========================================================================
type testExtraConfig struct {
	Port	int			`conf:"cmd=p|port"`
	Name	string		`conf:"cmd=n|name"`
}

var cmdTrailingExtra = []string {
	"-p=1000", "-n=A", "a", "b",  
}

var cmdAnyExtra = []string {
	"a", "-p=1000", "b", "-n", "A", "c", "d",  
}

func TestCmdLineExtra(t *testing.T) {
	var opts *ConfigOptions
	var cfg *Config
	var err error
	var config interface{}

	opts = &ConfigOptions{Args:cmdTrailingExtra}
	opts.AllowCmdLineTrailingExtra = true
	config = &testExtraConfig{}
	cfg, err = opts.Parse(config)
	if err != nil {
		t.Fatalf("***Error*** %v\n", err)
	}
	testeq(t, cfg.RemainingArgs(), []string{"a", "b"})

	opts = &ConfigOptions{Args:cmdAnyExtra}
	opts.AllowCmdLineAnyExtra = true
	config = &testExtraConfig{}
	cfg, err = opts.Parse(config)
	if err != nil {
		t.Fatalf("***Error*** %v\n", err)
	}
	testeq(t, cfg.RemainingArgs(), []string{"a", "b", "c", "d"})

	// Getting error
	opts = &ConfigOptions{Args:cmdTrailingExtra}
	opts.CmdLineError = ReturnError
	config = &testExtraConfig{}
	cfg, err = opts.Parse(config)
	if err == nil {
		t.Fatalf("***Error*** no error when expected\n")
	}
	testeq(t, cfg.RemainingArgs(), []string{"a", "b"})

	opts = &ConfigOptions{Args:cmdAnyExtra}
	opts.CmdLineError = ReturnError
	config = &testExtraConfig{}
	cfg, err = opts.Parse(config)
	if err == nil {
		t.Fatalf("***Error*** no error when expected\n")
	}
	testeq(t, cfg.RemainingArgs(), cmdAnyExtra)
}

//===========================================================================
// Test command-line slice
//===========================================================================
type testSliceConfig struct {
	Ports	[]int			`conf:"cmd=p|port"`
	Names	[]string		`conf:"cmd=n|name"`
}

var cmdSlice = []string {
	"--p=1,000", "-port=10,010", "-p", "100,200", 
	"-n=A", "--name", "B", "-name=C", 
}

func TestCmdLineSlice(t *testing.T) {
	model := &testSliceConfig{
			Ports:	[]int{1000, 10010, 100200}, 
			Names:	[]string{"A", "B", "C"}, 
		}

	opts := &ConfigOptions{Args:cmdSlice}
	config := &testSliceConfig{}
	_, err := opts.Parse(config)
	if err != nil {
		t.Fatalf("***Error*** %v\n", err)
	}

	testeq(t, config, model)
}

//===========================================================================
// Helpers
//===========================================================================
func testeq(t *testing.T, a interface{}, b interface{}) {
	if reflect.DeepEqual(a, b) == false {
		t.Fatalf("***Error*** Different %v:\n  a=%+v\n  b=%+v\n",
			reflect.ValueOf(a).Type(), a, b)
	}
}
	
func testnn(t *testing.T, a, b int64) {
	if a != b {
		t.Fatalf("***Error*** not expected value: a=%d b=%d\n", a, b)
	}
}


