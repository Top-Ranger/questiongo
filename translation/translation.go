package translation

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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
	WeekdayMonday               string
	WeekdayTuesday              string
	WeekdayWednesday            string
	WeekdayThursday             string
	WeekdayFriday               string
	WeekdaySaturday             string
	WeekdaySunday               string
}

const defaultLanguage = "en"

var fixedDefaultTranslation Translation

var current string
var languageMap = make(map[string]Translation)
var rwlock sync.RWMutex
var translationPath = "./translation"

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
	file = filepath.Join(translationPath, file)

	if _, err := os.Open(file); os.IsNotExist(err) {
		return Translation{}, fmt.Errorf("no translation for language '%s'", language)
	}

	b, err := ioutil.ReadFile(file)
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
