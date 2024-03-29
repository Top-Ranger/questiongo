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

// The main package holds the actual server of QuestionGo!
package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
	"github.com/Top-Ranger/questiongo/translation"
)

// ErrValidation represents an error related to validating answer input
type ErrValidation error

var questionnaireTemplate *template.Template
var questionnaireStartTemplate *template.Template

func init() {
	var err error
	questionnaireTemplate, err = template.New("questionnaire").Funcs(evenOddFuncMap).ParseFS(templateFiles, "template/questionnaire.html")
	if err != nil {
		panic(err)
	}

	questionnaireStartTemplate, err = template.ParseFS(templateFiles, "template/start.html")
	if err != nil {
		panic(err)
	}

}

// QuestionnairePage represents a single page on the questionnaire.
type QuestionnairePage struct {
	RandomOrderQuestions bool
	Questions            [][]string

	questions []registry.Question
}

// Questionnaire represents a questionnaire.
// It provides useful methods to handle the questionnaire.
// It must not be created on its own, but retrieved from LoadQuestionnaire or LoadAllQuestionnaires.
// A questionnaire is expected to hold all information in a single directory.
type Questionnaire struct {
	Password                  string
	PasswordMethod            string
	Open                      bool
	Language                  string
	Start                     string
	StartFormat               string
	End                       string
	EndFormat                 string
	Contact                   string
	RandomOrderPages          bool
	DoNotRandomiseFirstNPages int
	DoNotRandomiseLastNPages  int
	ShowProgress              bool
	AllowBack                 bool
	Pages                     []QuestionnairePage

	startCache   []byte
	endCache     []byte
	id           string
	allQuestions []registry.Question
}

type questionnaireTemplatePageStruct struct {
	QuestionData []template.HTML
	First        bool
	Last         bool
	NextID       string
	PrevID       string
	ID           string
}

type questionnaireTemplateStruct struct {
	Pages        []questionnaireTemplatePageStruct
	ShowProgress bool
	AllowBack    bool
	ID           string
	Translation  translation.Translation
	ServerPath   string
}

type questionnaireStartTemplateStruct struct {
	Text        template.HTML
	Key         string
	Contact     string
	Translation translation.Translation
	ServerPath  string
}

// GetStart returns the questionnaire start page.
func (q Questionnaire) GetStart() []byte {
	return q.startCache
}

// GetEnd returns the questionnaire end page.
func (q Questionnaire) GetEnd() []byte {
	return q.endCache
}

// WriteQuestions writes a html page containing the actual questionnaire to the writer.
// Since the questionnaite might contain random elements, it should be called seperately for each user instead of caching the result.
func (q Questionnaire) WriteQuestions(w io.Writer) {
	translationStruct, err := translation.GetTranslation(q.Language)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("can not get translation for language '%s'", q.Language)))
	}

	t := questionnaireTemplateStruct{
		Pages:        make([]questionnaireTemplatePageStruct, len(q.Pages)),
		ID:           q.id,
		ShowProgress: q.ShowProgress,
		AllowBack:    q.AllowBack,
		Translation:  translationStruct,
		ServerPath:   config.ServerPath,
	}
	for p := range q.Pages {
		questionData := make([]template.HTML, len(q.Pages[p].questions))
		for i := range q.Pages[p].questions {
			questionData[i] = q.Pages[p].questions[i].GetHTML()
		}
		if q.Pages[p].RandomOrderQuestions {
			rand.Shuffle(len(questionData), func(i, j int) {
				questionData[i], questionData[j] = questionData[j], questionData[i]
			})
		}
		t.Pages[p].QuestionData = questionData
	}

	if q.RandomOrderPages {
		rand.Shuffle(len(t.Pages)-q.DoNotRandomiseFirstNPages-q.DoNotRandomiseLastNPages, func(i, j int) {
			t.Pages[i+q.DoNotRandomiseFirstNPages], t.Pages[j+q.DoNotRandomiseFirstNPages] = t.Pages[j+q.DoNotRandomiseFirstNPages], t.Pages[i+q.DoNotRandomiseFirstNPages]
		})
	}

	for p := range t.Pages {
		t.Pages[p].ID = fmt.Sprintf("__page_%d", p)
		t.Pages[p].NextID = fmt.Sprintf("__page_%d", p+1)
		t.Pages[p].PrevID = fmt.Sprintf("__page_%d", p-1)
		if p == 0 {
			t.Pages[p].First = true
		}
		if p == len(t.Pages)-1 {
			t.Pages[p].Last = true
		}
	}

	err = questionnaireTemplate.ExecuteTemplate(w, "questionnaire.html", t)
	if err != nil {
		fmt.Println(err.Error())
	}
}

// GetResults returns a save html fragment containing the results of a question for each question.
func (q Questionnaire) GetResults() ([]template.HTML, error) {
	safe, ok := registry.GetDataSafe(config.DataSafe)
	if !ok {
		return nil, fmt.Errorf("can not get datasafe %s", config.DataSafe)
	}

	ids := make([]string, len(q.allQuestions))
	for i := range q.allQuestions {
		ids[i] = q.allQuestions[i].GetID()
	}

	data, err := safe.GetData(q.id, ids)
	if len(data) != len(ids) {
		return nil, fmt.Errorf("datasafe returned %d question data, expected was %d", len(data), len(ids))
	}
	if err != nil {
		return nil, err
	}

	result := make([]template.HTML, 0, len(q.allQuestions))

	for i := range q.allQuestions {
		result = append(result, q.allQuestions[i].GetStatisticsDisplay(data[i]))
	}

	return result, nil
}

// WriteZip writes a zip file containing one result file per question to the writer.
func (q Questionnaire) WriteZip(w io.Writer) error {
	safe, ok := registry.GetDataSafe(config.DataSafe)
	if !ok {
		return fmt.Errorf("can not get datasafe %s", config.DataSafe)
	}

	ids := make([]string, len(q.allQuestions))
	for i := range q.allQuestions {
		ids[i] = q.allQuestions[i].GetID()
	}

	data, err := safe.GetData(q.id, ids)
	if len(data) != len(ids) {
		return fmt.Errorf("datasafe returned %d question data, expected was %d", len(data), len(ids))
	}
	if err != nil {
		return err
	}

	result := zip.NewWriter(w)

	for i := range q.allQuestions {
		f, err := result.Create(strings.Join([]string{q.allQuestions[i].GetID(), "csv"}, "."))
		if err != nil {
			return err
		}
		csv := csv.NewWriter(f)

		err = csv.Write(q.allQuestions[i].GetStatisticsHeader())
		if err != nil {
			return err
		}

		if err != nil {
			return err
		}

		r := q.allQuestions[i].GetStatistics(data[i])
		err = csv.WriteAll(r)
		if err != nil {
			return csv.Error()
		}
	}

	return result.Close()
}

// WriteCSV writes a single csv file containing the current combined results of all questions.
func (q Questionnaire) WriteCSV(w io.Writer) error {
	safe, ok := registry.GetDataSafe(config.DataSafe)
	if !ok {
		return fmt.Errorf("can not get datasafe %s", config.DataSafe)
	}

	ids := make([]string, len(q.allQuestions))
	for i := range q.allQuestions {
		ids[i] = q.allQuestions[i].GetID()
	}

	data, err := safe.GetData(q.id, ids)
	if len(data) != len(ids) {
		return fmt.Errorf("datasafe returned %d question data, expected was %d", len(data), len(ids))
	}
	if err != nil {
		return err
	}

	csv := csv.NewWriter(w)

	header := make([]string, 0)
	result := make([][][]string, len(q.allQuestions))
	maxLength := 0
	for i := range q.allQuestions {
		header = append(header, q.allQuestions[i].GetStatisticsHeader()...)

		result[i] = q.allQuestions[i].GetStatistics(data[i])
		if len(result[i]) > maxLength {
			maxLength = len(result[i])
		}
	}

	err = csv.Write(helper.EscapeCSVLine(header))
	if err != nil {
		return err
	}

	showError := sync.Once{}
	errorList := make([]string, 0)

	for data := 0; data < maxLength; data++ {
		write := make([]string, 0, len(header))
		for i := range result {
			if len(result[i]) > data {
				write = append(write, result[i][data]...)
			} else {
				// Sone question has less result
				// This should not happen
				// Let's still catch this by filling it with empty data
				showError.Do(func() {
					t := translation.GetDefaultTranslation()
					log.Printf("csv export (%s): %s", q.id, t.ErrorAnswersDifferentAmount)
					errorList = append(errorList, t.ErrorAnswersDifferentAmount)
				})
				write = append(write, make([]string, len(q.allQuestions[i].GetStatisticsHeader()))...)
			}
		}
		csv.Write(helper.EscapeCSVLine(write))
	}

	csv.Flush()

	for i := range errorList {
		t := translation.GetDefaultTranslation()
		w.Write([]byte("\n#"))
		w.Write([]byte(t.AnErrorOccured))
		w.Write([]byte(": "))
		w.Write([]byte(errorList[i]))
	}

	return csv.Error()
}

// SaveData stores the questionnaire results contained in the http.Request permanently.
func (q Questionnaire) SaveData(r *http.Request) error {
	results := make(map[string]map[string][]string)
	safe, ok := registry.GetDataSafe(config.DataSafe)
	if !ok {
		return fmt.Errorf("can not get datasafe %s", config.DataSafe)
	}
	r.ParseForm()
	for k := range r.Form {
		split := strings.Split(k, "_")
		if len(split) == 0 {
			continue
		}
		ok := false
		for i := range q.allQuestions {
			if split[0] == q.allQuestions[i].GetID() {
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		m, ok := results[split[0]]
		if !ok {
			m = make(map[string][]string)
			results[split[0]] = m
		}
		m[k] = r.Form[k]
	}

	// Validate input first
	for i := range q.allQuestions {
		m, ok := results[q.allQuestions[i].GetID()]
		if !ok {
			m = make(map[string][]string)
		}
		err := q.allQuestions[i].ValidateInput(m)
		if err != nil {
			return ErrValidation(fmt.Errorf("save data: Validation failed for '%s - %s': %s", q.id, q.allQuestions[i].GetID(), err.Error()))
		}
	}

	// See if we need to drop the data
	for i := range q.allQuestions {
		m, ok := results[q.allQuestions[i].GetID()]
		if !ok {
			m = make(map[string][]string)
		}
		if q.allQuestions[i].IgnoreRecord(m) {
			// Silently drop out and ignore the record
			return nil
		}
	}

	questionID := make([]string, len(q.allQuestions))
	data := make([]string, len(q.allQuestions))
	for i := range q.allQuestions {
		m, ok := results[q.allQuestions[i].GetID()]
		if !ok {
			m = make(map[string][]string)
		}
		questionID[i] = q.allQuestions[i].GetID()
		data[i] = q.allQuestions[i].GetDatabaseEntry(m)
	}

	err := safe.SaveData(q.id, questionID, data)
	if err != nil {
		log.Printf("save data: Can not save questionnaire data for '%s': %s", q.id, err.Error())
	}

	return err
}

// LoadQuestionnaire loads a single questionnaire from a file.
// path must contain the path to the questionnaire folder.
// file must contain the path to the actual questionnaire json.
// key holds the key of the questionnaire (usually path).
func LoadQuestionnaire(path, file, key string) (Questionnaire, error) {
	// Load config
	var q Questionnaire
	b, err := os.ReadFile(file)
	if err != nil {
		return Questionnaire{}, err
	}
	err = json.Unmarshal(b, &q)
	if err != nil {
		return Questionnaire{}, err
	}

	translationStruct, err := translation.GetTranslation(q.Language)
	if err != nil {
		return Questionnaire{}, fmt.Errorf("can not get translation for language '%s'", q.Language)
	}

	// Check password method
	ok := registry.PasswordMethodExists(q.PasswordMethod)
	if !ok {
		return Questionnaire{}, fmt.Errorf("unknown password method '%s'", q.PasswordMethod)
	}

	// Load Questions
	testID := make(map[string]bool)
	q.allQuestions = make([]registry.Question, 0)
	for p := range q.Pages {
		q.Pages[p].questions = make([]registry.Question, 0, len(q.Pages[p].Questions))

		for i := range q.Pages[p].Questions {
			if len(q.Pages[p].Questions[i]) != 3 {
				return Questionnaire{}, fmt.Errorf("question %d-%d arguments have wrong length (%s)", p, i, file)
			}
			if strings.Contains(q.Pages[p].Questions[i][0], "_") {
				return Questionnaire{}, fmt.Errorf("ID %s must not have '_' (%s)", q.Pages[p].Questions[i][0], file)
			}
			if testID[q.Pages[p].Questions[i][0]] {
				return Questionnaire{}, fmt.Errorf("ID %s found twice (%s)", q.Pages[p].Questions[i][0], file)
			}
			testID[q.Pages[p].Questions[i][0]] = true
			pathQ := filepath.Join(path, q.Pages[p].Questions[i][2])
			b, err = os.ReadFile(pathQ)
			if err != nil {
				return Questionnaire{}, fmt.Errorf("can not read file %s: %w (%s)", pathQ, err, file)
			}
			factory, ok := registry.GetQuestionType(q.Pages[p].Questions[i][1])
			if !ok {
				return Questionnaire{}, fmt.Errorf("unknown question type %s (%s)", q.Pages[p].Questions[i][1], file)
			}
			newQuestion, err := factory(b, q.Pages[p].Questions[i][0], q.Language)
			if err != nil {
				return Questionnaire{}, fmt.Errorf("can not create question %d-%d: %w (%s)", p, i, err, file)
			}
			q.Pages[p].questions = append(q.Pages[p].questions, newQuestion)
			q.allQuestions = append(q.allQuestions, newQuestion)
		}
	}

	// Fill cache
	pathQ := filepath.Join(path, q.Start)
	b, err = os.ReadFile(pathQ)
	if err != nil {
		return Questionnaire{}, fmt.Errorf("can not read file %s: %w (%s)", pathQ, err, file)
	}
	f, ok := registry.GetFormatType(q.StartFormat)
	if !ok {
		return Questionnaire{}, fmt.Errorf("can not format start: Unknown type %s (%s)", q.StartFormat, file)
	}
	td := questionnaireStartTemplateStruct{
		Text:        f.Format(b),
		Key:         key,
		Contact:     q.Contact,
		Translation: translationStruct,
		ServerPath:  config.ServerPath,
	}
	output := bytes.NewBuffer(make([]byte, 0, len(td.Text)+len(td.Contact)+5000))
	questionnaireStartTemplate.Execute(output, td)
	q.startCache = output.Bytes()

	pathQ = filepath.Join(path, q.End)
	b, err = os.ReadFile(pathQ)
	if err != nil {
		return Questionnaire{}, fmt.Errorf("can not read file %s: %w (%s)", pathQ, err, file)
	}
	f, ok = registry.GetFormatType(q.EndFormat)
	if !ok {
		return Questionnaire{}, fmt.Errorf("can not format end: Unknown type %s (%s)", q.StartFormat, file)
	}
	text := textTemplateStruct{f.Format(b), translationStruct, config.ServerPath}
	output = bytes.NewBuffer(make([]byte, 0, len(text.Text)*2))
	textTemplate.Execute(output, text)
	q.endCache = output.Bytes()

	// Check random order
	if q.RandomOrderPages {
		if q.DoNotRandomiseFirstNPages < 0 {
			return Questionnaire{}, fmt.Errorf("value DoNotRandomiseFirstNPages must be positive, is %d (%s)", q.DoNotRandomiseFirstNPages, file)
		}
		if q.DoNotRandomiseLastNPages < 0 {
			return Questionnaire{}, fmt.Errorf("value DoNotRandomiseLastNPages must be positive, is %d (%s)", q.DoNotRandomiseLastNPages, file)
		}

		if q.DoNotRandomiseFirstNPages+q.DoNotRandomiseLastNPages > len(q.Pages) {
			return Questionnaire{}, fmt.Errorf("DoNotRandomiseFirstNPages + DoNotRandomiseLastNPages must not be larger than number of pages, currently %d + %d = %d > %d (%s)", q.DoNotRandomiseFirstNPages, q.DoNotRandomiseLastNPages, q.DoNotRandomiseFirstNPages+q.DoNotRandomiseLastNPages, len(q.Pages), file)
		}
	}

	// ID
	q.id = key

	return q, nil
}

// LoadAllQuestionnaires loads all questionnaires from a folder.
// It expects to have each questionnaire in a direct subfolder.
// The questionnaire definition is in that subfolder in the file 'questionnaire.json'.
func LoadAllQuestionnaires(dataPath string) (map[string]Questionnaire, error) {
	questionnaires := make(map[string]Questionnaire)

	dirs, err := os.ReadDir(config.DataFolder)
	if err != nil {
		return nil, err
	}

	for i := range dirs {
		if !dirs[i].IsDir() {
			continue
		}
		content, err := os.ReadDir(filepath.Join(config.DataFolder, dirs[i].Name()))
		if err != nil {
			continue
		}
		for j := range content {
			if content[j].Name() == "questionnaire.json" {
				q, err := LoadQuestionnaire(filepath.Join(config.DataFolder, dirs[i].Name()), filepath.Join(config.DataFolder, dirs[i].Name(), content[j].Name()), dirs[i].Name())
				if err != nil {
					log.Printf("load all questionnaire: Can not load %s: %s", filepath.Join(config.DataFolder, dirs[i].Name(), content[j].Name()), err.Error())
					break
				}
				questionnaires[dirs[i].Name()] = q
				break
			}
		}
	}
	return questionnaires, nil
}
