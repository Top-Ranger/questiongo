// SPDX-License-Identifier: Apache-2.0
// Copyright 2020,2025 Marcus Soll
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
	"strings"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactoryBipolarMatrix, "bipolar matrix")
	if err != nil {
		panic(err)
	}
}

// FactoryBipolarMatrix is the factory for matrix questions.
func FactoryBipolarMatrix(data []byte, id string, language string) (registry.Question, error) {
	var m bipolarmatrix
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}
	m.id = id

	// Sanity checks
	testID := make(map[string]bool)
	for i := range m.AnswerIDs {
		if testID[m.AnswerIDs[i]] {
			return nil, fmt.Errorf("bipolarmatrix: ID %s found twice (%s)", m.AnswerIDs[i], id)
		}
		testID[m.AnswerIDs[i]] = true
	}

	testID = make(map[string]bool)
	for i := range m.Questions {
		if len(m.Questions[i]) != 3 {
			return nil, fmt.Errorf("bipolarmatrix: Answer %d must have exactly 3 values (id, low text, high text) (%s)", i, id)
		}
		if testID[m.Questions[i][0]] {
			return nil, fmt.Errorf("bipolarmatrix: ID %s found twice (%s)", m.Questions[i][0], id)
		}
		testID[m.Questions[i][0]] = true
	}

	_, ok := registry.GetFormatType(m.Format)
	if !ok {
		return nil, fmt.Errorf("bipolarmatrix: Unknown format type %s (%s)", m.Format, id)
	}

	return &m, nil
}

var bipolarmatrixTemplate = template.Must(template.New("matrixTemplate").Parse(`{{.Title}}<br>
<table>
<tbody>
{{range $i, $e := .Data }}
<tr>
<td><strong>{{$e.Low}}</strong></td>
{{range $I, $E := $.AnswerIDs }}
<td class="centre" onclick="document.getElementById('{{$.GID}}_{{$e.QID}}_{{$E}}').checked=true;"><input type="radio" id="{{$.GID}}_{{$e.QID}}_{{$E}}" name="{{$.GID}}_{{$e.QID}}" value="{{$E}}" {{if $.Required}} required {{end}}></td>
{{end}}
<td><strong>{{$e.High}}</strong></td>
</tr>
{{end}}
</tbody>
</table>
`))

var bipolarmatrixStatisticsTemplate = template.Must(template.New("matrixStatisticTemplate").Parse(`{{.Title}}<br>
<table>
<thead>
<tr>
<th></th>
{{range $i, $e := .Header }}
<th>{{$e}}</th>
{{end}}
<th></th>
</tr>
</thead>
{{range $i, $e := .Data }}
<tr>
<td><strong>{{$e.Low}}</strong></td>
{{range $I, $E := $e.Result }}
<td>{{printf "%.2f" $E}}</td>
{{end}}
<td><strong>{{$e.High}}</strong></td>
</tr>
{{end}}
</tbody>
</table>
{{.Image}}
`))

type bipolarmatrixTemplateStructInner struct {
	Low  template.HTML
	High template.HTML
	QID  string
}

type bipolarmatrixTemplateStruct struct {
	Title     template.HTML
	Required  bool
	Data      []bipolarmatrixTemplateStructInner
	AnswerIDs []string
	GID       string
}

type bipolarmatrixStatisticsTemplateStructInner struct {
	Low    template.HTML
	High   template.HTML
	Result []float64
}

type bipolarmatrixStatisticTemplateStruct struct {
	Title  template.HTML
	Header []template.HTML
	Data   []bipolarmatrixStatisticsTemplateStructInner
	Image  template.HTML
}

type bipolarmatrix struct {
	Random    bool
	Required  bool
	Format    string
	Title     string
	AnswerIDs []string
	Questions [][]string

	id string
}

func (m bipolarmatrix) GetID() string {
	return m.id
}

func (m bipolarmatrix) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(m.Format)
	td := bipolarmatrixTemplateStruct{
		Title:     f.Format([]byte(m.Title)),
		Required:  m.Required,
		Data:      make([]bipolarmatrixTemplateStructInner, 0, len(m.Questions)),
		AnswerIDs: m.AnswerIDs,
		GID:       m.id,
	}
	for i := range m.Questions {
		mts := bipolarmatrixTemplateStructInner{
			QID:  m.Questions[i][0],
			Low:  f.FormatClean([]byte(m.Questions[i][1])),
			High: f.FormatClean([]byte(m.Questions[i][2])),
		}
		td.Data = append(td.Data, mts)
	}

	if m.Random {
		rand.Shuffle(len(td.Data), func(i, j int) {
			td.Data[i], td.Data[j] = td.Data[j], td.Data[i]
		})
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := bipolarmatrixTemplate.Execute(output, td)
	if err != nil {
		log.Printf("bipolarmatrix: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (m bipolarmatrix) GetStatisticsHeader() []string {
	header := make([]string, len(m.Questions))
	for i := range m.Questions {
		header[i] = fmt.Sprintf("%s_%s", m.id, m.Questions[i][0])
	}
	return header
}

func (m bipolarmatrix) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for d := range data {
		r := make([]string, len(m.Questions))
		errorState := true
		if !strings.HasPrefix(data[d], "ERROR") {
			rarray := make([]string, len(m.AnswerIDs))
			err := json.Unmarshal([]byte(data[d]), &rarray)
			if err == nil && len(rarray) == len(r) {
				errorState = false
				for i := range rarray {
					for j := range m.AnswerIDs {
						if rarray[i] == m.AnswerIDs[j] {
							r[i] = m.AnswerIDs[j]
							break
						}
					}
				}
			}
		}

		if errorState {
			for i := range r {
				r[i] = "error"
			}
		}
		result[d] = r
	}
	return result
}

func (m bipolarmatrix) GetStatisticsDisplay(data []string) template.HTML {
	count := 0
	countAnswer := make([][]int, len(m.Questions))
	for i := range m.Questions {
		countAnswer[i] = make([]int, len(m.AnswerIDs)+1)
	}

	for d := range data {
		rarray := make([]string, len(m.Questions))
		err := json.Unmarshal([]byte(data[d]), &rarray)
		if err != nil {
			continue
		}
		if len(rarray) != len(m.Questions) {
			continue
		}
		count++

		for i := range m.Questions {
			found := false
			for j := range m.AnswerIDs {
				if rarray[i] == m.AnswerIDs[j] {
					countAnswer[i][j]++
					found = true
					break
				}
			}
			if !found {
				countAnswer[i][len(m.AnswerIDs)]++
			}
		}
	}

	f, _ := registry.GetFormatType(m.Format)
	td := bipolarmatrixStatisticTemplateStruct{
		Title:  f.Format([]byte(m.Title)),
		Header: make([]template.HTML, len(m.AnswerIDs)+1),
		Data:   make([]bipolarmatrixStatisticsTemplateStructInner, 0, len(m.Questions)),
		Image:  "",
	}
	labelValues := make([]string, len(m.AnswerIDs))
	labelBars := make([]string, len(m.Questions))
	for i := range m.AnswerIDs {
		td.Header[i] = f.FormatClean([]byte(m.AnswerIDs[i]))
		labelValues[i] = string(td.Header[i])
	}
	td.Header[len(m.AnswerIDs)] = "[no answer]"
	v := make([][]int, 0, len(m.Questions))

	for i := range m.Questions {
		vinner := make([]int, len(m.AnswerIDs))
		low := f.FormatClean([]byte(m.Questions[i][1]))
		high := f.FormatClean([]byte(m.Questions[i][2]))
		inner := bipolarmatrixStatisticsTemplateStructInner{
			Low:    low,
			High:   high,
			Result: make([]float64, len(m.AnswerIDs)+1),
		}
		labelBars[i] = string(fmt.Sprintf("%s - %s", string(low), string(high)))
		for j := range m.AnswerIDs {
			inner.Result[j] = float64(countAnswer[i][j]) / float64(count)
			vinner[j] = countAnswer[i][j]
		}
		inner.Result[len(m.AnswerIDs)] = float64(countAnswer[i][len(m.AnswerIDs)]) / float64(count)

		td.Data = append(td.Data, inner)
		v = append(v, vinner)
	}
	td.Image = helper.Stacked100Chart(v, fmt.Sprintf("%s_bar", m.id), labelBars, labelValues, "")
	output := bytes.NewBuffer(make([]byte, 0))
	err := bipolarmatrixStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("bipolarmatrix: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (m bipolarmatrix) ValidateInput(data map[string][]string) error {
	for i := range m.Questions {
		r, ok := data[fmt.Sprintf("%s_%s", m.id, m.Questions[i][0])]
		if ok && len(r) >= 1 {
			found := false
			for j := range m.AnswerIDs {
				if r[0] == m.AnswerIDs[j] {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("bipolarmatrix: Unknown id '%s' for question '%s'", r[0], fmt.Sprintf("%s_%s", m.id, m.Questions[i][0]))
			}
		} else {
			if m.Required {
				return fmt.Errorf("bipolarmatrix: '%s' required, but no input found", fmt.Sprintf("%s_%s", m.id, m.Questions[i][0]))
			}
		}
	}
	return nil
}

func (m bipolarmatrix) IgnoreRecord(data map[string][]string) bool {
	return false
}

func (m bipolarmatrix) GetDatabaseEntry(data map[string][]string) string {
	result := make([]string, len(m.Questions))
	for i := range m.Questions {
		r, ok := data[fmt.Sprintf("%s_%s", m.id, m.Questions[i][0])]
		if ok && len(r) >= 1 {
			result[i] = r[0]
		} else {
			result[i] = ""
		}
	}
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err.Error())
	}
	return string(b)
}
