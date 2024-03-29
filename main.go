// SPDX-License-Identifier: Apache-2.0
// Copyright 2020,2022 Marcus Soll
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	// Register types
	_ "github.com/Top-Ranger/questiongo/datasafe"
	_ "github.com/Top-Ranger/questiongo/format"
	_ "github.com/Top-Ranger/questiongo/passwordmethods"
	_ "github.com/Top-Ranger/questiongo/question"
	"github.com/Top-Ranger/questiongo/registry"
	"github.com/Top-Ranger/questiongo/translation"
)

// Config represents the configuration of QuestionGo!
type Config struct {
	Language              string
	Address               string
	PathImpressum         string
	FormatImpressum       string
	PathDSGVO             string
	FormatDSGVO           string
	DataFolder            string
	DataSafe              string
	DataSafeConfig        string
	LogFailedLogin        bool
	ServerPath            string
	ReloadPasswordsMethod string
	ReloadPasswords       []string

	reloadingDisabled bool
}

var config Config

func loadConfig(path string) (Config, error) {
	log.Printf("main: Loading config (%s)", path)
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, errors.New(fmt.Sprintln("Can not read config.json:", err))
	}

	c := Config{}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return Config{}, errors.New(fmt.Sprintln("Error while parsing config.json:", err))
	}

	if !strings.HasPrefix(c.ServerPath, "/") && c.ServerPath != "" {
		log.Println("load config: ServerPath does not start with '/', adding it as a prefix")
		c.ServerPath = strings.Join([]string{"/", c.ServerPath}, "")
	}
	c.ServerPath = strings.TrimSuffix(c.ServerPath, "/")

	if c.ReloadPasswordsMethod != "" && len(c.ReloadPasswords) != 0 {
		ok := registry.PasswordMethodExists(c.ReloadPasswordsMethod)
		if !ok {
			return c, errors.New(fmt.Sprintln("Unknown password method for reload:", c.ReloadPasswordsMethod))
		}
	} else {
		log.Println("load config: disabling reloading")
		c.reloadingDisabled = true
	}

	return c, nil
}

func printInfo() {
	log.Println("QuestionGo!")
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		log.Print("- no build info found")
		return
	}

	log.Printf("- go version: %s", bi.GoVersion)
	for _, s := range bi.Settings {
		switch s.Key {
		case "-tags":
			log.Printf("- build tags: %s", s.Value)
		case "vcs.revision":
			l := 7
			if len(s.Value) > 7 {
				s.Value = s.Value[:l]
			}
			log.Printf("- commit: %s", s.Value)
		case "vcs.modified":
			log.Printf("- files modified: %s", s.Value)
		}
	}
}

func main() {
	printInfo()
	rand.Seed(time.Now().Unix())

	configPath := flag.String("config", "./config/config.json", "Path to json config for QuestionGo!")
	flag.Parse()

	c, err := loadConfig(*configPath)
	if err != nil {
		panic(err)
	}
	config = c

	err = translation.SetDefaultTranslation(config.Language)
	if err != nil {
		log.Panicf("main: Error setting default language '%s': %s", config.Language, err.Error())
	}
	log.Printf("main: Setting language to '%s'", config.Language)

	datasafe, ok := registry.GetDataSafe(config.DataSafe)
	if !ok {
		log.Panicf("main: Unknown data safe %s", config.DataSafe)
	}

	b, err := os.ReadFile(config.DataSafeConfig)
	if err != nil {
		log.Panicln(err)
	}

	err = datasafe.LoadConfig(b)
	if err != nil {
		log.Panicln(err)
	}

	RunServer()

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)

	log.Println("main: waiting")

	for range s {
		StopServer()
		datasafe.FlushAndClose()
		return
	}
}
