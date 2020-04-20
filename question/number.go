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
	"strconv"
	"strings"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactoryNumber, "number")
	if err != nil {
		panic(err)
	}
}

// FactoryNumber is the factory for number questions.
func FactoryNumber(data []byte, id string) (registry.Question, error) {
	var n numberQuestion
	err := json.Unmarshal(data, &n)
	if err != nil {
		return nil, err
	}
	n.id = id

	if n.HasMinMax {
		if n.Max < n.Min {
			return nil, fmt.Errorf("number: max (%d) must be larger than min (%d) (%s)", n.Max, n.Min, id)
		}
	}

	if n.HasStep {
		if n.Step < 1 {
			return nil, fmt.Errorf("number: step (%d) must be at least 1", n.Step)
		}

		if n.HasMinMax {
			if n.Step > n.Max-n.Min {
				return nil, fmt.Errorf("number: step (%d) must be smaller than the range (%d) (%s)", n.Step, n.Max-n.Min, id)
			}
		} else {
			return nil, fmt.Errorf("number: step can not be used without min / max")
		}
	}

	_, ok := registry.GetFormatType(n.Format)
	if !ok {
		return nil, fmt.Errorf("number: Unknown format type %s (%s)", n.Format, id)
	}

	return &n, nil
}

var numberTemplate = template.Must(template.New("numberTemplate").Parse(`<label for="{{.QID}}">{{.Question}}</label><br>
<input type="number" name="{{.QID}}" {{if .HasMinMax}}min="{{.Min}}" max="{{.Max}}"{{end}} {{if .HasStep}}step="{{.Step}}"{{end}} {{if .Required}}required{{end}}>
`))

var numberStatisticsTemplate = template.Must(template.New("numberStatisticTemplate").Parse(`{{.Question}}<br>
<table>
<thead>
<tr>
<th>Value</th>
<th>Number</th>
</tr>
</thead>
{{range $i, $e := .Data }}
<tr>
<td>{{$e.Value}}</td>
<td>{{$e.Number}}</td>
</tr>
{{end}}
<tr>
<td class="th-cell">[average]</td>
<td>{{printf "%.2f" .Average}}</td>
</tr>
<tr>
<td class="th-cell">[number answer]</td>
<td>{{.Count}}</td>
</tr>
<tr>
<td class="th-cell">[no answer]</td>
<td>{{.NoAnswer}}</td>
</tr>
<tr>
<td class="th-cell">[invalid input]</td>
<td>{{.Invalid}}</td>
</tr>
</tbody>
</table>
<br>
{{.Image}}
`))

type numberTemplateStruct struct {
	Question  template.HTML
	QID       string
	Required  bool
	HasMinMax bool
	Min       int
	Max       int
	HasStep   bool
	Step      int
}

type numberStatisticTemplateStructInner struct {
	Value  int
	Number int
}

type numberStatisticTemplateStruct struct {
	Question template.HTML
	Data     []numberStatisticTemplateStructInner
	Average  float64
	Count    int
	Invalid  int
	NoAnswer int
	Image    template.HTML
}
type numberStatisticTemplateStructInnerSort []numberStatisticTemplateStructInner

func (n numberStatisticTemplateStructInnerSort) Len() int {
	return len(n)
}

func (n numberStatisticTemplateStructInnerSort) Less(i, j int) bool {
	return n[i].Value < n[j].Value
}

func (n numberStatisticTemplateStructInnerSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

type numberQuestion struct {
	Format    string
	Question  string
	Required  bool
	HasMinMax bool
	Min       int
	Max       int
	HasStep   bool
	Step      int

	id string
}

func (n numberQuestion) GetID() string {
	return n.id
}

func (n numberQuestion) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(n.Format)

	td := numberTemplateStruct{
		Question:  f.Format([]byte(n.Question)),
		QID:       n.id,
		Required:  n.Required,
		HasMinMax: n.HasMinMax,
		Min:       n.Min,
		Max:       n.Max,
		HasStep:   n.HasStep,
		Step:      n.Step,
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := numberTemplate.Execute(output, td)
	if err != nil {
		log.Printf("number: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (n numberQuestion) GetStatisticsHeader() []string {
	return []string{n.id}
}

func (n numberQuestion) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for i := range data {
		if data[i] == "" {
			result[i] = []string{""}
			continue
		}

		value, err := strconv.Atoi(data[i])

		if err != nil {
			result[i] = []string{strings.Join([]string{"[invalid input]", err.Error()}, " ")}
		} else if n.HasMinMax && value < n.Min || value > n.Max {
			result[i] = []string{fmt.Sprintf("[invalid input] value %d out of range", value)}
		} else {
			result[i] = []string{data[i]}
		}
	}
	return result
}

func (n numberQuestion) GetStatisticsDisplay(data []string) template.HTML {
	f, _ := registry.GetFormatType(n.Format)

	td := numberStatisticTemplateStruct{
		Question: f.Format([]byte(n.Question)),
		Average:  0.0,
		Count:    0,
		Invalid:  0,
		NoAnswer: 0,
	}

	answer := make(map[int]int)

	for i := range data {
		if data[i] == "" {
			td.NoAnswer++
			continue
		}
		value, err := strconv.Atoi(data[i])
		if err != nil {
			td.Invalid++
		} else {
			td.Count++
			answer[value]++
			td.Average += float64(value)
		}
	}

	for k := range answer {
		td.Data = append(td.Data, numberStatisticTemplateStructInner{Value: k, Number: answer[k]})
	}

	sort.Sort(numberStatisticTemplateStructInnerSort(td.Data))

	v := make([]helper.ChartValue, len(td.Data)+1)

	for i := range td.Data {
		v[i].Label = strconv.Itoa(td.Data[i].Value)
		v[i].Value = float64(td.Data[i].Number)
	}

	v[len(td.Data)].Label = "[no answer]"
	v[len(td.Data)].Value = float64(td.NoAnswer)

	td.Image = helper.BarChart(v, n.id, string(f.FormatClean([]byte(n.Question))))

	td.Average /= float64(td.Count)

	output := bytes.NewBuffer(make([]byte, 0))
	err := numberStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("number: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (n numberQuestion) ValidateInput(data map[string][]string) error {
	if len(data[n.id]) == 0 || data[n.id][0] == "" {
		if n.Required {
			return fmt.Errorf("number (%s): No input found", n.id)
		}
		return nil
	}
	value, err := strconv.Atoi(data[n.id][0])
	if err != nil {
		return fmt.Errorf("number: Input '%s' malformed (%s)", data[n.id][0], err.Error())
	}

	if n.HasMinMax {
		i := n.Min
		for i <= n.Max {
			if i == value {
				return nil
			}
			if n.HasStep {
				i += n.Step
			} else {
				i++
			}
		}
		return fmt.Errorf("number: Input '%d' not in range", value)
	}
	return nil
}

func (n numberQuestion) GetDatabaseEntry(data map[string][]string) string {
	if len(data[n.id]) >= 1 {
		return data[n.id][0]
	}
	return ""
}
