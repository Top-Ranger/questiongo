// SPDX-License-Identifier: Apache-2.0
// Copyright 2020 Marcus Soll
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
	"crypto/sha512"
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Top-Ranger/questiongo/registry"
)

var questionnaireTemplate *template.Template
var questionnaireStartTemplate *template.Template

var questionnairePasswordHash = func(s string) []byte {
	hash := sha512.Sum512([]byte(s))
	return hash[:]
}

func init() {
	funcMap := template.FuncMap{
		"even": func(i int) bool {
			return i%2 == 0
		},
	}

	b, err := ioutil.ReadFile("template/questionnaire.html")
	if err != nil {
		panic(err)
	}

	questionnaireTemplate, err = template.New("questionnaire").Funcs(funcMap).Parse(string(b))
	if err != nil {
		panic(err)
	}

	b, err = ioutil.ReadFile("template/start.html")
	if err != nil {
		panic(err)
	}

	questionnaireStartTemplate, err = template.New("questionnaireStart").Parse(string(b))
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
	Password         string
	Open             bool
	Start            string
	StartFormat      string
	End              string
	EndFormat        string
	Contact          string
	RandomOrderPages bool
	ShowProgress     bool
	AllowBack        bool
	Pages            []QuestionnairePage

	startCache   []byte
	endCache     []byte
	id           string
	hash         []byte
	allQuestions []registry.Question
	saveMutex    *sync.Mutex // Ensure saving does not mix up
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
}

type questionnaireStartTemplateStruct struct {
	Text    template.HTML
	Key     string
	Contact string
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
	t := questionnaireTemplateStruct{
		Pages:        make([]questionnaireTemplatePageStruct, len(q.Pages)),
		ID:           q.id,
		ShowProgress: q.ShowProgress,
		AllowBack:    q.AllowBack,
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
		rand.Shuffle(len(t.Pages), func(i, j int) {
			t.Pages[i], t.Pages[j] = t.Pages[j], t.Pages[i]
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

	err := questionnaireTemplate.Execute(w, t)
	if err != nil {
		fmt.Println(err.Error())
	}
}

// GetResults returns a save html fragment containing the results of a question for each question.
func (q Questionnaire) GetResults() ([]template.HTML, error) {
	safe, ok := registry.GetDataSafe(config.DataSafe)
	if !ok {
		return nil, fmt.Errorf("Can not get datasafe %s", config.DataSafe)
	}

	result := make([]template.HTML, 0, len(q.allQuestions))

	for i := range q.allQuestions {
		data, err := safe.GetData(q.id, q.allQuestions[i].GetID())
		if err != nil {
			return nil, err
		}

		result = append(result, q.allQuestions[i].GetStatisticsDisplay(data))
	}

	return result, nil
}

// WriteZip writes a zip file containing one result file per question to the writer.
func (q Questionnaire) WriteZip(w io.Writer) error {
	safe, ok := registry.GetDataSafe(config.DataSafe)
	if !ok {
		return fmt.Errorf("Can not get datasafe %s", config.DataSafe)
	}

	result := zip.NewWriter(w)

	q.saveMutex.Lock()
	defer q.saveMutex.Unlock()
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

		data, err := safe.GetData(q.id, q.allQuestions[i].GetID())
		if err != nil {
			return err
		}

		r := q.allQuestions[i].GetStatistics(data)
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
		return fmt.Errorf("Can not get datasafe %s", config.DataSafe)
	}

	csv := csv.NewWriter(w)

	header := make([]string, 0)
	result := make([][][]string, len(q.allQuestions))
	maxLength := 0

	q.saveMutex.Lock()
	defer q.saveMutex.Unlock()
	for i := range q.allQuestions {
		header = append(header, q.allQuestions[i].GetStatisticsHeader()...)

		data, err := safe.GetData(q.id, q.allQuestions[i].GetID())
		if err != nil {
			return err
		}

		result[i] = q.allQuestions[i].GetStatistics(data)
		if len(result[i]) > maxLength {
			maxLength = len(result[i])
		}
	}

	err := csv.Write(header)
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
					log.Printf("csv export (%s): Not all questions have the same number of results (only reported once for each export)", q.id)
					errorList = append(errorList, "Not all questions have the same number of results data. Results might be inconsistent (only reported once for each export)")
				})
				write = append(write, make([]string, len(q.allQuestions[i].GetStatisticsHeader()))...)
			}
		}
		csv.Write(write)
	}

	csv.Flush()

	for i := range errorList {
		w.Write([]byte("\n#An error occured: "))
		w.Write([]byte(errorList[i]))
	}

	return csv.Error()
}

// SaveData stores the questionnaire results contained in the http.Request permanently.
func (q Questionnaire) SaveData(r *http.Request) error {
	results := make(map[string]map[string][]string)
	safe, ok := registry.GetDataSafe(config.DataSafe)
	if !ok {
		return fmt.Errorf("Can not get datasafe %s", config.DataSafe)
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

	// We need to ensure the order is preserved and that parallel execution does not mess it up
	q.saveMutex.Lock()
	defer q.saveMutex.Unlock()
	for i := range q.allQuestions {
		m, ok := results[q.allQuestions[i].GetID()]
		if !ok {
			m = make(map[string][]string)
		}
		s := q.allQuestions[i].GetDatabaseEntry(m)
		err := safe.SaveData(q.id, q.allQuestions[i].GetID(), s)
		if err != nil {
			log.Printf("save data: Can not save questionnaire data for %s - %s: %s", q.id, q.allQuestions[i].GetID(), err.Error())
		}
	}

	return nil
}

// VerifyPassword verifies whether the provided password matches the questionnaire password.
func (q Questionnaire) VerifyPassword(password string) bool {
	hash := questionnairePasswordHash(password)
	r := subtle.ConstantTimeCompare(hash, q.hash) // for security
	return r == 1
}

// LoadQuestionnaire loads a single questionnaire from a file.
// path must contain the path to the questionnaire folder.
// file must contain the path to the actual questionnaire json.
// key holds the key of the questionnaire (usually path).
func LoadQuestionnaire(path, file, key string) (Questionnaire, error) {
	// Load config
	var q Questionnaire
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return Questionnaire{}, err
	}
	err = json.Unmarshal(b, &q)
	if err != nil {
		return Questionnaire{}, err
	}

	// Load Questions
	testID := make(map[string]bool)
	q.allQuestions = make([]registry.Question, 0)
	for p := range q.Pages {
		q.Pages[p].questions = make([]registry.Question, 0, len(q.Pages[p].Questions))

		for i := range q.Pages[p].Questions {
			if len(q.Pages[p].Questions[i]) != 3 {
				return Questionnaire{}, fmt.Errorf("Question %d-%d arguments have wrong length (%s)", p, i, file)
			}
			if strings.Contains(q.Pages[p].Questions[i][0], "_") {
				return Questionnaire{}, fmt.Errorf("ID %s must not have '_' (%s)", q.Pages[p].Questions[i][0], file)
			}
			_, ok := testID[q.Pages[p].Questions[i][0]]
			if ok {
				return Questionnaire{}, fmt.Errorf("ID %s found twice (%s)", q.Pages[p].Questions[i][0], file)
			}
			testID[q.Pages[p].Questions[i][0]] = true
			pathQ := filepath.Join(path, q.Pages[p].Questions[i][2])
			b, err = ioutil.ReadFile(pathQ)
			if err != nil {
				return Questionnaire{}, fmt.Errorf("Can not read file %s: %w (%s)", pathQ, err, file)
			}
			factory, ok := registry.GetQuestionType(q.Pages[p].Questions[i][1])
			if !ok {
				return Questionnaire{}, fmt.Errorf("Unknown question type %s (%s)", q.Pages[p].Questions[i][1], file)
			}
			newQuestion, err := factory(b, q.Pages[p].Questions[i][0])
			if err != nil {
				return Questionnaire{}, fmt.Errorf("Can not create question %d-%d: %w (%s)", p, i, err, file)
			}
			q.Pages[p].questions = append(q.Pages[p].questions, newQuestion)
			q.allQuestions = append(q.allQuestions, newQuestion)
		}
	}

	// Fill cache
	pathQ := filepath.Join(path, q.Start)
	b, err = ioutil.ReadFile(pathQ)
	if err != nil {
		return Questionnaire{}, fmt.Errorf("Can not read file %s: %w (%s)", pathQ, err, file)
	}
	f, ok := registry.GetFormatType(q.StartFormat)
	if !ok {
		return Questionnaire{}, fmt.Errorf("Can not format start: Unknown type %s (%s)", q.StartFormat, file)
	}
	td := questionnaireStartTemplateStruct{
		Text:    f.Format(b),
		Key:     key,
		Contact: q.Contact,
	}
	output := bytes.NewBuffer(make([]byte, 0, len(td.Text)+len(td.Contact)+5000))
	questionnaireStartTemplate.Execute(output, td)
	q.startCache = output.Bytes()

	pathQ = filepath.Join(path, q.End)
	b, err = ioutil.ReadFile(pathQ)
	if err != nil {
		return Questionnaire{}, fmt.Errorf("Can not read file %s: %w (%s)", pathQ, err, file)
	}
	f, ok = registry.GetFormatType(q.EndFormat)
	if !ok {
		return Questionnaire{}, fmt.Errorf("Can not format end: Unknown type %s (%s)", q.StartFormat, file)
	}
	text := textTemplateStruct{f.Format(b)}
	output = bytes.NewBuffer(make([]byte, 0, len(text.Text)*2))
	textTemplate.Execute(output, text)
	q.endCache = output.Bytes()

	// ID
	q.id = key

	// hash
	q.hash = questionnairePasswordHash(q.Password)

	// mutex
	q.saveMutex = new(sync.Mutex)

	return q, nil
}

// LoadAllQuestionnaires loads all questionnaires from a folder.
// It expects to have each questionnaire in a direct subfolder.
// The questionnaire definition is in that subfolder in the file 'questionnaire.json'.
func LoadAllQuestionnaires(dataPath string) (map[string]Questionnaire, error) {
	questionnaires := make(map[string]Questionnaire)

	dirs, err := ioutil.ReadDir(config.DataFolder)
	if err != nil {
		return nil, err
	}

	for i := range dirs {
		if !dirs[i].IsDir() {
			continue
		}
		content, err := ioutil.ReadDir(filepath.Join(config.DataFolder, dirs[i].Name()))
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
