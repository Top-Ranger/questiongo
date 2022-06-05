// SPDX-License-Identifier: Apache-2.0
// Copyright 2020,2021,2022 Marcus Soll
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

package translation

import (
	"embed"
	"encoding/json"
	"log"
	"reflect"
	"strings"
	"sync"
)

// Translation represents an object holding all translations
type Translation struct {
	Language                    string
	CreatedBy                   string
	Impressum                   string
	PrivacyPolicy               string
	PreviousPage                string
	NextPage                    string
	FinishQuestionnaire         string
	QuestionnaireProgress       string
	Key                         string
	Password                    string
	AccessResults               string
	JavaScriptRequired          string
	StartQuestionnaire          string
	ResponsiblePerson           string
	CanNotFindQuestionnaire     string
	QuestionnaireClosed         string
	ErrorAnswer                 string
	AnErrorOccured              string
	ErrorAnswersDifferentAmount string
	AppointmentYes              string
	AppointmentNo               string
	AppointmentOnlyIfNeeded     string
	AppointmentCanNotSay        string
	AppointmentName             string
	AppointmentRequired         string
	AppointmentOptional         string
	AppointmentComment          string
	AppointmentAll              string
	WeekdayMonday               string
	WeekdayTuesday              string
	WeekdayWednesday            string
	WeekdayThursday             string
	WeekdayFriday               string
	WeekdaySaturday             string
	WeekdaySunday               string
	ReloadSurveys               string
}

const defaultLanguage = "en"

//go:embed *.json
var translationFiles embed.FS

var fixedDefaultTranslation Translation

var current string
var languageMap = make(map[string]Translation)
var rwlock sync.RWMutex

func init() {
	err := SetDefaultTranslation(defaultLanguage)
	if err != nil {
		log.Printf("Can not load default language (%s): %s", defaultLanguage, err.Error())
	}

	fixedDefaultTranslation = GetDefaultTranslation()
	if err != nil {
		panic(err)
	}
}

// GetTranslation returns a Translation struct of the given language.
func GetTranslation(language string) (Translation, error) {
	if language == "" {
		return GetDefaultTranslation(), nil
	}

	rwlock.RLock()
	t, ok := languageMap[language]
	rwlock.RUnlock()
	if ok {
		// We don't need to reload translation
		return t, nil
	}

	rwlock.Lock()
	defer rwlock.Unlock()

	file := strings.Join([]string{language, "json"}, ".")

	b, err := translationFiles.ReadFile(file)
	if err != nil {
		return Translation{}, err
	}
	t = Translation{}
	err = json.Unmarshal(b, &t)
	if err != nil {
		return Translation{}, err
	}

	// Set unknown strings to default value
	vp := reflect.ValueOf(&t)
	dv := reflect.ValueOf(fixedDefaultTranslation)
	v := vp.Elem()

	for i := 0; i < v.NumField(); i++ {
		if !v.Field(i).CanSet() {
			continue
		}
		if v.Field(i).Kind() != reflect.String {
			continue
		}
		if v.Field(i).String() == "" {
			v.Field(i).SetString(dv.Field(i).String())
		}
	}

	languageMap[language] = t
	return t, nil
}

// SetDefaultTranslation sets the default language to the provided one.
// Does nothing if it returns error != nil.
func SetDefaultTranslation(language string) error {
	if language == "" {
		return nil
	}

	// Just load it in cache and ensure no error occures
	_, err := GetTranslation(language)
	// Since those are locked in GetTranslation, we need to load the language first before locking mutex
	rwlock.Lock()
	defer rwlock.Unlock()
	if err != nil {
		return err
	}
	current = language
	return nil
}

// GetDefaultTranslation returns a Translation struct of the current default language.
func GetDefaultTranslation() Translation {
	rwlock.RLock()
	defer rwlock.RUnlock()
	return languageMap[current]
}
