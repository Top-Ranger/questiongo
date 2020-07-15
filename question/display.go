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

package question

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"

	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactoryDisplay, "display")
	if err != nil {
		panic(err)
	}
}

// FactoryDisplay is the factory for displays pseudo-question.
func FactoryDisplay(data []byte, id string, language string) (registry.Question, error) {
	var d display
	err := json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}
	d.id = id

	_, ok := registry.GetFormatType(d.Format)
	if !ok {
		return nil, fmt.Errorf("display: Unknown format type %s (%s)", d.Format, id)
	}

	return &d, nil
}

type display struct {
	Format string
	Text   string

	id string
}

func (d display) GetID() string {
	return d.id
}

func (d display) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(d.Format)

	formatted := string(f.Format([]byte(d.Text)))
	return template.HTML(strings.Join([]string{"<div>", formatted, "</div>"}, "\n"))
}

func (d display) GetStatisticsHeader() []string {
	return []string{}
}

func (d display) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for i := range data {
		result[i] = []string{}
	}
	return result
}

func (d display) GetStatisticsDisplay(data []string) template.HTML {
	f, _ := registry.GetFormatType(d.Format)

	formatted := string(f.Format([]byte(d.Text)))
	return template.HTML(strings.Join([]string{"<div>", formatted, "</div>", "<p><em>Display has no results</em></p>"}, "\n"))
}

func (d display) ValidateInput(data map[string][]string) error {
	return nil
}

func (d display) IgnoreRecord(data map[string][]string) bool {
	return false
}

func (d display) GetDatabaseEntry(data map[string][]string) string {
	return ""
}
