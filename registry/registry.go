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

// Package registry provides a central way to register and use all available formatting options, saving backends, and question types.
// All options should be registered prior to the program starting, normally through init()
// Since the questionnaires are handled as immutable, it does not make much sense to register options later.
package registry

import (
	"html/template"
	"sync"
)

// AlreadyRegisteredError represents an error where an option is already registeres
type AlreadyRegisteredError string

// Error returns the error description
func (a AlreadyRegisteredError) Error() string {
	return string(a)
}

// QuestionFactory represents a function to generate a new Question object from the input.
// The input can be question specific.
type QuestionFactory func(data []byte, id string, language string) (Question, error)

// Question represents a single question.
// The results of a question is collected through a normal HTML form, so all questions must provide their results appropriately.
// The names and ids must start with the provided id. If more is needed, they must add a '_' after the id, and then arbitrary identifier.
// It is assumed that all questions can be trusted.
// All methods must be save for parallel usage.
type Question interface {

	// GetID returns the ID of the question
	GetID() string

	// GetHTML returns the HTML representation of the question.
	// The fragmen must be HTML safe, input name must start with QuestionID_. HTML ids must follow the same rule.
	GetHTML() template.HTML

	// GetStatisticsHeader returns the name of the provided question result headers.
	GetStatisticsHeader() []string

	// GetStatistics returns the result from the question.
	// Each slice entry must contain a list of all data in the same length and order as the header.
	// data holds all database entries currently available.
	GetStatistics(data []string) [][]string

	// GetStatisticsDisplay returns a HTML fragment representing the current results.
	// data holds all database entries currently available.
	GetStatisticsDisplay(data []string) template.HTML

	// ValidateInput validates whether the given data can be considered valid (e.g. all required input is there).
	// The method must return error != nil if the input is not valid.
	// The method must return error == nil if the input is valid.
	ValidateInput(data map[string][]string) error

	// GetDatabaseEntry returns a string representation of the results of the question.
	// The data map returns the values of the POST request of the client, filtered by questions.
	GetDatabaseEntry(data map[string][]string) string
}

// Format represents a formatting option.
// All methods must be save for parallel usage.
type Format interface {
	Format(b []byte) template.HTML
	FormatClean(b []byte) template.HTML
}

// DataSafe represents a backend for save storage of questionnaire results.
// All results must be stored in the same order they are added, grouped by questionnaireID and questionID.
// However, there reordering is allowed as long as the order for one questionnaireID / questionID combination is retained.
// All methods must be save for parallel usage.
type DataSafe interface {
	SaveData(questionnaireID, questionID, data string) error // Must preserve the order of data for a questionnaireID, questionID combination
	GetData(questionnaireID, questionID string) ([]string, error)
	LoadConfig(data []byte) error
	FlushAndClose()
}

var (
	knownQuestionTypes      = make(map[string]QuestionFactory)
	knownQuestionTypesMutex = sync.RWMutex{}
	knownFormatTypes        = make(map[string]Format)
	knownFormatTypesMutex   = sync.RWMutex{}
	knownDataSafes          = make(map[string]DataSafe)
	knownDataSafesMutex     = sync.RWMutex{}
)

// RegisterQuestionType registeres a question type.
// The name of the question type is used as an identifier and must be unique.
// You can savely use it in parallel.
func RegisterQuestionType(f QuestionFactory, name string) error {
	knownQuestionTypesMutex.Lock()
	defer knownQuestionTypesMutex.Unlock()

	_, ok := knownQuestionTypes[name]
	if ok {
		return AlreadyRegisteredError("Question already registered")
	}
	knownQuestionTypes[name] = f
	return nil
}

// GetQuestionType returns a question type.
// The bool indicates whether it existed. You can only use it if the bool is true.
func GetQuestionType(name string) (QuestionFactory, bool) {
	knownQuestionTypesMutex.RLock()
	defer knownQuestionTypesMutex.RUnlock()
	f, ok := knownQuestionTypes[name]
	return f, ok
}

// RegisterFormatType registeres a format type.
// The name of the format type is used as an identifier and must be unique.
// You can savely use it in parallel.
func RegisterFormatType(t Format, name string) error {
	knownFormatTypesMutex.Lock()
	defer knownFormatTypesMutex.Unlock()

	_, ok := knownFormatTypes[name]
	if ok {
		return AlreadyRegisteredError("Format already registered")
	}
	knownFormatTypes[name] = t
	return nil
}

// GetFormatType returns a format type.
// The bool indicates whether it existed. You can only use it if the bool is true.
func GetFormatType(name string) (Format, bool) {
	knownFormatTypesMutex.RLock()
	defer knownFormatTypesMutex.RUnlock()
	f, ok := knownFormatTypes[name]
	return f, ok
}

// RegisterDataSafe registeres a data safe.
// The name of the data safe is used as an identifier and must be unique.
// You can savely use it in parallel.
func RegisterDataSafe(t DataSafe, name string) error {
	knownDataSafesMutex.Lock()
	defer knownDataSafesMutex.Unlock()

	_, ok := knownDataSafes[name]
	if ok {
		return AlreadyRegisteredError("DataSafe already registered")
	}
	knownDataSafes[name] = t
	return nil
}

// GetDataSafe returns a data safe.
// The bool indicates whether it existed. You can only use it if the bool is true.
func GetDataSafe(name string) (DataSafe, bool) {
	knownDataSafesMutex.RLock()
	defer knownDataSafesMutex.RUnlock()
	f, ok := knownDataSafes[name]
	return f, ok
}
