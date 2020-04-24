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
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactoryDisplayRandomGroup, "display random group")
	if err != nil {
		panic(err)
	}
}

// FactoryDisplayRandomGroup is the factory for random group seperation by showing different displays.
func FactoryDisplayRandomGroup(data []byte, id string, language string) (registry.Question, error) {
	var drg displayRandomGroup
	err := json.Unmarshal(data, &drg)
	if err != nil {
		return nil, err
	}
	drg.id = id

	testID := make(map[string]bool)
	for i := range drg.Text {
		if len(drg.Text[i]) != 2 {
			return nil, fmt.Errorf("display random group: Group %d must have exactly 2 values (id, text) (%s)", i, id)
		}
		if testID[drg.Text[i][0]] {
			return nil, fmt.Errorf("display random group: ID %s found twice (%s)", drg.Text[i][0], id)
		}
		testID[drg.Text[i][0]] = true
	}

	_, ok := registry.GetFormatType(drg.Format)
	if !ok {
		return nil, fmt.Errorf("display random group: Unknown format type %s (%s)", drg.Format, id)
	}

	return &drg, nil
}

var displayRandomGroupTemplate = template.Must(template.New("displayRandomGroupTemplate").Parse(`{{.Text}}
<input type="hidden" id="{{.QID}}_{{.AID}}" name="{{.QID}}" value="{{.AID}}">
`))

var displayRandomGroupStatisticsTemplate = template.Must(template.New("displayRandomGroupStatisticsTemplate").Parse(`<p>{{.ID}}</p><br>
<table>
<thead>
<tr>
<th>Group</th>
<th>Number</th>
</tr>
</thead>
<tbody>
{{range $i, $e := .Data }}
<tr>
<td>{{$e.Group}}</td>
<td>{{$e.Result}}</td>
</tr>
{{end}}
</tbody>
</table>
<br>
{{.Image}}
`))

type displayRandomGroupTemplateStruct struct {
	QID  string
	AID  string
	Text template.HTML
}

type displayRandomGroupStatisticsTemplateStruct struct {
	ID    string
	Data  []displayRandomGroupStatisticsTemplateStructInner
	Image template.HTML
}

type displayRandomGroupStatisticsTemplateStructInner struct {
	Group  string
	Result int
}

type displayRandomGroup struct {
	Format string
	Text   [][]string

	id string
}

func (drg displayRandomGroup) GetID() string {
	return drg.id
}

func (drg displayRandomGroup) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(drg.Format)

	group := rand.Intn(len(drg.Text))
	td := displayRandomGroupTemplateStruct{
		QID:  drg.id,
		AID:  drg.Text[group][0],
		Text: f.Format([]byte(drg.Text[group][1])),
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := displayRandomGroupTemplate.Execute(output, td)
	if err != nil {
		log.Printf("display random group: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (drg displayRandomGroup) GetStatisticsHeader() []string {
	return []string{drg.id}
}

func (drg displayRandomGroup) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for i := range data {
		result[i] = []string{data[i]}
	}
	return result
}

func (drg displayRandomGroup) GetStatisticsDisplay(data []string) template.HTML {
	countAnswer := make([]int, len(drg.Text))

	for d := range data {
		for i := range drg.Text {
			if data[d] == drg.Text[i][0] {
				countAnswer[i]++
				break
			}
		}
	}

	td := displayRandomGroupStatisticsTemplateStruct{
		ID:   drg.id,
		Data: make([]displayRandomGroupStatisticsTemplateStructInner, 0, len(drg.Text)),
	}
	v := make([]helper.ChartValue, len(drg.Text))

	for i := range drg.Text {
		v[i].Label = string(drg.Text[i][0])
		v[i].Value = float64(countAnswer[i])
		inner := displayRandomGroupStatisticsTemplateStructInner{
			Group:  drg.Text[i][0],
			Result: countAnswer[i],
		}
		td.Data = append(td.Data, inner)
	}

	td.Image = helper.PieChart(v, drg.id, drg.id)

	output := bytes.NewBuffer(make([]byte, 0))
	err := displayRandomGroupStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("display random group: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (drg displayRandomGroup) ValidateInput(data map[string][]string) error {
	r, ok := data[drg.id]
	if !ok || len(r) == 0 {
		return fmt.Errorf("display random group: No input found")
	}
	for i := range drg.Text {
		if r[0] == drg.Text[i][0] {
			return nil
		}
	}
	return fmt.Errorf("display random group: Unknown group '%s'", r[0])
}

func (drg displayRandomGroup) GetDatabaseEntry(data map[string][]string) string {
	r, ok := data[drg.id]
	if !ok || len(r) == 0 {
		return ""
	}
	return r[0]
}
