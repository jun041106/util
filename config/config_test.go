// Copyright 2012 Apcera Inc. All rights reserved.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type CommonConfig struct {
	URL		string
	Port	int32		`conf:"cmd=p|port,default=4222"`
}

type DbConfig struct {
	URL			string
	Database	string
	Username	string
}

type TestConfig struct {
	
	CommonConfig
	
	Debug	bool	`conf:"cmd=d|debug"`
	Trace	bool	`conf:"cmd=t|trace"`
	Verbose	bool	`conf:"cmd=v|verbose"`
	Level	int		`conf:"default=99"`
	PInt	*int	`conf:"cmd=z"`
	Name	string	`conf:"default=apcera"`
	
	Db		*DbConfig
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

	var err error
	
	fileName := filepath.Join(os.TempDir(),
		fmt.Sprintf("apcera_config_test.%d", time.Now().UnixNano()))

	os.Remove(fileName)
	defer os.Remove(fileName)

	err = writeFile(fileName, config_text)		
	if err != nil {
		t.Fatalf("***Error*** returned from Parse: %v\n", err)
	}

	opts := &ConfigOptions{Help:help,DefaultConfigFile:fileName}
	
	opts.Args = cmdLineSample
	
	config := &TestConfig{}
	
	var cfg *Config
	
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


