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
	"sort"
	"strings"
	"time"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactoryDate, "date")
	if err != nil {
		panic(err)
	}
}

// FactoryDate is the factory for date questions.
func FactoryDate(data []byte, id string) (registry.Question, error) {
	var d dateQuestion
	err := json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}
	d.id = id

	_, ok := registry.GetFormatType(d.Format)
	if !ok {
		return nil, fmt.Errorf("date: Unknown format type %s (%s)", d.Format, id)
	}

	return &d, nil
}

var dateTemplate = template.Must(template.New("dateTemplate").Parse(`<label for="{{.QID}}">{{.Question}}</label><br>
<input type="date" id="{{.QID}}" name="{{.QID}}" placeholder="yyyy-mm-dd" pattern="^\d{4}-\d{2}-\d{2}$" {{if .Required}} required {{end}}>
`))

var dateStatisticsTemplate = template.Must(template.New("dateStatisticTemplate").Parse(`{{.Question}}<br>
<table>
<thead>
<tr>
<th>Date</th>
<th>Number</th>
</tr>
</thead>
<tbody>
{{range $i, $e := .Data }}
<tr>
<td {{if $e.Special}}class="th-cell"{{end}}>{{$e.Date}}</td>
<td>{{$e.Number}}</td>
</tr>
{{end}}
</tbody>
</table>
<br>
{{.Image}}
`))

type dateTemplateStruct struct {
	Question template.HTML
	QID      string
	Required bool
}

type dateStatisticTemplateStructInner struct {
	Date    string
	Number  int
	Special bool
}

type dateStatisticTemplateStruct struct {
	Question template.HTML
	Data     []dateStatisticTemplateStructInner
	Image    template.HTML
}
type dateStatisticTemplateStructInnerSort []dateStatisticTemplateStructInner

func (d dateStatisticTemplateStructInnerSort) Len() int {
	return len(d)
}

func (d dateStatisticTemplateStructInnerSort) Less(i, j int) bool {
	return d[i].Date < d[j].Date
}

func (d dateStatisticTemplateStructInnerSort) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

type dateQuestion struct {
	Format   string
	Question string
	Required bool

	id string
}

func (d dateQuestion) GetID() string {
	return d.id
}

func (d dateQuestion) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(d.Format)

	td := dateTemplateStruct{
		Question: f.Format([]byte(d.Question)),
		QID:      d.id,
		Required: d.Required,
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := dateTemplate.Execute(output, td)
	if err != nil {
		log.Printf("date: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (d dateQuestion) GetStatisticsHeader() []string {
	return []string{d.id}
}

func (d dateQuestion) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for i := range data {
		result[i] = []string{data[i]}
	}
	return result
}

func (d dateQuestion) GetStatisticsDisplay(data []string) template.HTML {
	f, _ := registry.GetFormatType(d.Format)
	answer := make(map[string]int)

	for i := range data {
		if data[i] == "" {
			answer["[no answer]"]++
		} else if strings.HasPrefix(data[i], "[invalid input]") {
			answer["[invalid input]"]++
		} else {
			answer[data[i]]++
		}
	}

	td := dateStatisticTemplateStruct{
		Question: f.Format([]byte(d.Question)),
		Data:     make([]dateStatisticTemplateStructInner, 0, len(answer)),
	}

	for k := range answer {
		td.Data = append(td.Data, dateStatisticTemplateStructInner{Date: k, Number: answer[k], Special: strings.HasPrefix(string(k), "[")})
	}

	sort.Sort(dateStatisticTemplateStructInnerSort(td.Data))

	v := make([]helper.ChartValue, len(td.Data))
	for i := range td.Data {
		v[i].Label = td.Data[i].Date
		v[i].Value = float64(td.Data[i].Number)
	}

	td.Image = helper.BarChart(v, d.id, string(f.FormatClean([]byte(d.Question))))

	output := bytes.NewBuffer(make([]byte, 0))
	err := dateStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("date: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (d dateQuestion) GetDatabaseEntry(data map[string][]string) string {
	if len(data[d.id]) >= 1 {
		if data[d.id][0] == "" {
			return ""
		}
		// Validate Date
		_, err := time.Parse("2006-01-02", data[d.id][0])
		if err == nil {
			return data[d.id][0]
		}
		return strings.Join([]string{"[invalid input]", err.Error()}, " ")
	}
	return ""
}
