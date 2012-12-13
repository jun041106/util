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
	Port	int32		`cfg:"name=p:port,default=4222"`
}

type DbConfig struct {
	URL			string
	Database	string
	Username	string
}

type TestConfig struct {
	
	CommonConfig
	
	Debug	bool	`cfg:"name=d:debug"`
	Trace	bool	`cfg:"name=t:trace"`
	Verbose	bool	`cfg:"name=v:verbose"`
	Level	int		`cfg:"default=99"`
	Name	string	`cfg:"default=apcera"`
	
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
	"-Level", "7777",
}

var config_text =
`
{
	"Level"	: 987654,
	"Db"	: { "URL" : "mysql://host:port", "foo" : 123 }
}
`

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
	
	_, err = opts.Parse(config)
	if err != nil {
		t.Fatalf("***Error*** returned from Parse: %v\n", err)
	}
	
	//fmt.Printf("----------------------------------------------\n")
	//fmt.Printf("Config after parsed:\n%+v\n", config)
	//fmt.Printf("  Db: %+v\n", config.Db)
	//fmt.Printf("----------------------------------------------\n")
}


