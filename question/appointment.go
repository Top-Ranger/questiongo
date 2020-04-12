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
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Top-Ranger/questiongo/registry"
)

var appointmentDateFormatRead = "2006-01-02"
var appointmentDateFormatWrite = "Monday, 02.01.2006 15:04"
var appointmentDateFormatWriteNoTime = "Monday, 02.01.2006"
var appointmentDateFormatID = "02.01.2006T15:04"
var appointmentDateFormatIDNoTime = "02.01.2006"

func init() {
	err := registry.RegisterQuestionType(FactoryAppointment, "appointment")
	if err != nil {
		panic(err)
	}
}

type appointmentSort [][]int

func (a appointmentSort) Len() int {
	return len(a)
}

func (a appointmentSort) Less(i, j int) bool {
	if a[i][0] < a[j][0] {
		return true
	}
	if a[i][0] > a[j][0] {
		return false
	}
	return a[i][1] < a[j][1]
}

func (a appointmentSort) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// FactoryAppointment is the factory for appointment questions.
func FactoryAppointment(data []byte, id string) (registry.Question, error) {
	var a appointment
	err := json.Unmarshal(data, &a)
	if err != nil {
		return nil, err
	}
	a.id = id

	_, ok := registry.GetFormatType(a.Format)
	if !ok {
		return nil, fmt.Errorf("appointment: Unknown format type %s (%s)", a.Format, id)
	}

	fd, err := time.Parse(appointmentDateFormatRead, a.FirstDate)
	if err != nil {
		return nil, fmt.Errorf("appointment: can not parse '%s' - %s", a.FirstDate, err.Error())
	}
	ld, err := time.Parse(appointmentDateFormatRead, a.LastDate)
	if err != nil {
		return nil, fmt.Errorf("appointment: can not parse '%s' - %s", a.LastDate, err.Error())
	}

	if ld.Before(fd) {
		return nil, fmt.Errorf("appointment: LastDate (%s) can not be before FirstDate (%s)", a.LastDate, a.FirstDate)
	}

	ld = ld.AddDate(0, 0, 1) // Add one day for the for-loop later

	w := make(map[time.Weekday]bool)
	for i := range a.Days {
		day, ok := appointmentParseWeekday(a.Days[i])
		if !ok {
			return nil, fmt.Errorf("appointment: can not parse day of week '%s'", a.Days[i])
		}
		w[day] = true
	}

	t := make([][]int, 0)

	test := make(map[string]bool)
	for i := range a.Time {
		if a.Time[i] == "notime" {
			if test["notime"] {
				return nil, fmt.Errorf("appointment: time '%s' found twice", a.Time[i])
			}
			test["notime"] = true
			t = append(t, []int{-1, -1})
			continue
		}

		tn := make([]int, 2)
		split := strings.Split(a.Time[i], ":")
		if len(split) != 2 {
			return nil, fmt.Errorf("appointment: Can not parse '%s' as time", a.Time[i])
		}
		tn[0], err = strconv.Atoi(split[0])
		if err != nil {
			return nil, fmt.Errorf("appointment: Can not parse '%s' as time - %s", a.Time[i], err.Error())
		}
		tn[1], err = strconv.Atoi(split[1])
		if err != nil {
			return nil, fmt.Errorf("appointment: Can not parse '%s' as time - %s", a.Time[i], err.Error())
		}

		if tn[0] < 0 || tn[0] > 23 {
			return nil, fmt.Errorf("appointment: Hour of time '%s' must be between 0 and 23", a.Time[i])
		}

		if tn[1] < 0 || tn[1] > 59 {
			return nil, fmt.Errorf("appointment: Minute of time '%s' must be between 0 and 59", a.Time[i])
		}

		// Ensure time format is identical
		timeTest := fmt.Sprintf("%d:%d", tn[0], tn[1])
		if test[timeTest] {
			return nil, fmt.Errorf("appointment: time '%s' found twice", a.Time[i])
		}
		test[timeTest] = true

		t = append(t, tn)
	}

	sort.Sort(appointmentSort(t))

	ignore := make([]time.Time, len(a.ExceptDays))

	for i := range a.ExceptDays {
		ignore[i], err = time.Parse(appointmentDateFormatRead, a.ExceptDays[i])
		if err != nil {
			return nil, fmt.Errorf("appointment: can not parse '%s' - %s", a.ExceptDays[i], err.Error())
		}
	}

	a.dates = make([]appointmentDate, 0)
	sort.Strings(a.Time)

	for fd.Before(ld) {
		for i := range t {
			var newTime time.Time

			if t[i][0] == -1 {
				// Special value "notime"
				newTime = time.Date(fd.Year(), fd.Month(), fd.Day(), 23, 59, 59, 999999999, fd.Location())
			} else {
				newTime = time.Date(fd.Year(), fd.Month(), fd.Day(), t[i][0], t[i][1], 0, 0, fd.Location())
			}

			add := w[newTime.Weekday()]
			for ign := range ignore {
				add = add && (newTime.Year() != ignore[ign].Year() || newTime.Month() != ignore[ign].Month() || newTime.Day() != ignore[ign].Day())
			}

			if add {
				if t[i][0] == -1 {
					// Special value "notime"
					a.dates = append(a.dates, appointmentDate{
						ID:      fmt.Sprintf("%s_%s_notime", id, newTime.Format(appointmentDateFormatIDNoTime)),
						Display: newTime.Format(appointmentDateFormatWriteNoTime),
						time:    newTime,
					})
				} else {
					a.dates = append(a.dates, appointmentDate{
						ID:      fmt.Sprintf("%s_%s", id, newTime.Format(appointmentDateFormatID)),
						Display: newTime.Format(appointmentDateFormatWrite),
						time:    newTime,
					})
				}
			}
		}
		fd = fd.AddDate(0, 0, 1)
	}

	return &a, nil
}

var appointmentDayMap = map[string]time.Weekday{
	"mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday, "sun": time.Sunday,
	"mo": time.Monday, "tu": time.Tuesday, "we": time.Wednesday, "th": time.Thursday, "fr": time.Friday, "sa": time.Saturday, "su": time.Sunday,
	"monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday, "thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday, "sunday": time.Sunday,
}

func appointmentParseWeekday(day string) (time.Weekday, bool) {
	day = strings.ToLower(day)
	weekday, ok := appointmentDayMap[day]
	return weekday, ok
}

func appointmentColour(answer string) string {
	switch answer {
	case "✓":
		return "#5EFF5E"
	case "👎":
		return "#FFE75E"
	case "X":
		return "#FF5E66"
	case "?":
		return "#DBD9E2"
	default:
		return ""
	}
}

func appointmentPoints(answer string) float64 {
	switch answer {
	case "✓":
		return 1.0
	case "👎":
		return 0.25
	case "X":
		return -1.0
	default:
		return 0.0
	}
}

var appointmentTemplate = template.Must(template.New("appointmentTemplate").Parse(`{{.Text}}<br>
<p><label for="{{.ID}}_name">Name {{if .NameRequired}}<em>(required)</em>{{else}}<em>(optional)</em>{{end}}:</label> <input type="text" id="{{.ID}}_name" name="{{.ID}}_name" placeholder="Name" maxlength="150" {{if .NameRequired}}required{{end}}></p>
<table>
<thead>
<tr>
<th></th>
<th>✓ (yes)</th>
<th>👎 (only if needed)</th>
<th>X (no)</th>
<th>? (can not say)</th>
</tr>
</thead>
<tr>
<td></td>
<td class="centre" bgcolor="#5EFF5E"><button form="detach from form" onclick="e=document.getElementById('{{.ID}}_tbody');l=e.getElementsByTagName('input');for(var i=0;i<l.length;i++){if(l[i].type==='radio'&&l[i].value==='✓'){l[i].checked=true}}">all ✓</button></td>
<td class="centre" bgcolor="#FFE75E"><button form="detach from form" onclick="e=document.getElementById('{{.ID}}_tbody');l=e.getElementsByTagName('input');for(var i=0;i<l.length;i++){if(l[i].type==='radio'&&l[i].value==='👎'){l[i].checked=true}}">all 👎</button></td>
<td class="centre" bgcolor="#FF5E66"><button form="detach from form" onclick="e=document.getElementById('{{.ID}}_tbody');l=e.getElementsByTagName('input');for(var i=0;i<l.length;i++){if(l[i].type==='radio'&&l[i].value==='X'){l[i].checked=true}}">all X</button></td>
<td class="centre" bgcolor="#DBD9E2"><button form="detach from form" onclick="e=document.getElementById('{{.ID}}_tbody');l=e.getElementsByTagName('input');for(var i=0;i<l.length;i++){if(l[i].type==='radio'&&l[i].value==='?'){l[i].checked=true}}">all ?</button></td>
</tr>
<tbody id="{{.ID}}_tbody">
{{range $i, $e := .Data }}
<tr>
<td>{{if $e.Disabled}}<s>{{else}}<strong>{{end}}{{$e.Display}}{{if $e.Disabled}}</s>{{else}}</strong>{{end}}</td>
<td class="centre" bgcolor="#5EFF5E" title="{{$e.Display}} - ✓"><input title="{{$e.Display}} - ✓" type="radio" name="{{$e.ID}}" value="✓" {{if $e.Disabled}} disabled {{end}}></td>
<td class="centre" bgcolor="#FFE75E" title="{{$e.Display}} - 👎"><input title="{{$e.Display}} - 👎" type="radio" name="{{$e.ID}}" value="👎" {{if $e.Disabled}} disabled {{end}}></td>
<td class="centre" bgcolor="#FF5E66" title="{{$e.Display}} - X"><input title="{{$e.Display}} - X" type="radio" name="{{$e.ID}}" value="X" {{if $e.Disabled}} disabled {{end}}></td>
<td class="centre" bgcolor="#DBD9E2" title="{{$e.Display}} - ?"><input title="{{$e.Display}} - ?" type="radio" name="{{$e.ID}}" value="?" {{if $e.Disabled}} disabled {{end}}></td>
</tr>
{{end}}
</tbody>
</table>
<p><label for="{{.ID}}_comment">Comment <em>(optional)</em>:</label> <input type="text" id="{{.ID}}_comment" name="{{.ID}}_comment" placeholder="Comment" maxlength="500"></p>
`))

var appointmentStatisticsTemplate = template.Must(template.New("appointmentStatisticsTemplate").Parse(`{{.Text}}
<p><strong>Best Date:</strong> {{.Best}}</p>
<details>
<summary>detailed results</summary>
<div style="width: 100%; overflow-x: auto;">
<table style="overflow-x: auto;">
<thead>
<tr>
<th></th>
{{range $i, $e := .Dates }}
<th>{{$e}}</th>
{{end}}
</tr>
</thead>
<tbody>
{{range $i, $e := .Data }}
<tr>
<td style="white-space:nowrap;">{{if $e.Comment}}<abbr title="{{$e.Comment}}">{{end}}<strong>{{$e.Name}}</strong>{{if $e.Comment}}</abbr>{{end}}</td>
{{range $I, $E := .Answers }}
<td class="centre" {{if index $E 0}}bgcolor="{{index $E 0}}"{{end}}>{{index $E 1}}</td>
{{end}}
</tr>
{{end}}
<tr>
<td class="th-cell" style="white-space:nowrap;"><strong>Points</strong></td>
{{range $i, $e := .Points }}
<td class="centre{{if eq $i $.BestNumber}} th-cell{{end}}">{{printf "%.2f" $e}}</td>
{{end}}
</tr>
</tbody>
</table>
</div>
</details>
`))

type appointmentTemplateStruct struct {
	ID           string
	Text         template.HTML
	NameRequired bool
	Data         []appointmentDate
}

type appointmentStatisticsTemplateStruct struct {
	Text       template.HTML
	Best       string
	Dates      []string
	Data       []appointmentStatisticsTemplateStructInner
	Points     []float64
	BestNumber int
}

type appointmentStatisticsTemplateStructInner struct {
	Name    string
	Comment string
	Answers [][]string // [colour, value]
}

type appointmentDate struct {
	ID       string
	Display  string
	Disabled bool
	time     time.Time
}

type appointment struct {
	Format              string
	Text                string
	NameRequired        bool
	DisallowVotesInPast bool
	FirstDate           string
	LastDate            string
	Days                []string
	Time                []string
	ExceptDays          []string

	id    string
	dates []appointmentDate
}

func (a appointment) GetID() string {
	return a.id
}

func (a appointment) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(a.Format)

	td := appointmentTemplateStruct{
		ID:           a.id,
		Text:         f.Format([]byte(a.Text)),
		NameRequired: a.NameRequired,
		Data:         make([]appointmentDate, len(a.dates)),
	}

	now := time.Now()

	for i := range a.dates {
		td.Data[i].ID = a.dates[i].ID
		td.Data[i].Display = a.dates[i].Display
		td.Data[i].Disabled = a.DisallowVotesInPast && a.dates[i].time.Before(now)
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := appointmentTemplate.Execute(output, td)
	if err != nil {
		log.Printf("date: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (a appointment) GetStatisticsHeader() []string {
	s := make([]string, len(a.dates)+2)
	s[0] = fmt.Sprintf("%s_name", a.id)
	s[len(a.dates)+1] = fmt.Sprintf("%s_comment", a.id)
	for i := range a.dates {
		s[i+1] = a.dates[i].ID
	}
	return s
}

func (a appointment) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for i := range data {
		s := make([]string, len(a.dates)+2)
		var results map[string]string
		err := json.Unmarshal([]byte(data[i]), &results)
		if err != nil {
			log.Printf("appointment: Error unmarshalling %d - %s - %s", i, data[i], err.Error())
			result[i] = s
			continue
		}
		s[0] = results[fmt.Sprintf("%s_name", a.id)]
		s[len(a.dates)+1] = results[fmt.Sprintf("%s_comment", a.id)]
		for j := range a.dates {
			s[j+1] = results[a.dates[j].ID]
		}
		result[i] = s
	}
	return result
}

func (a appointment) GetStatisticsDisplay(data []string) template.HTML {
	f, _ := registry.GetFormatType(a.Format)

	td := appointmentStatisticsTemplateStruct{
		Text:   f.Format([]byte(a.Text)),
		Dates:  make([]string, len(a.dates)),
		Data:   make([]appointmentStatisticsTemplateStructInner, 0, len(data)),
		Points: make([]float64, len(a.dates)),
	}

	for i := range a.dates {
		td.Dates[i] = a.dates[i].Display
	}

	for d := range data {
		var results map[string]string
		err := json.Unmarshal([]byte(data[d]), &results)
		if err != nil {
			log.Printf("appointment: Error unmarshalling %d - %s - %s", d, data[d], err.Error())
			continue
		}

		inner := appointmentStatisticsTemplateStructInner{
			Name:    results[fmt.Sprintf("%s_name", a.id)],
			Comment: results[fmt.Sprintf("%s_comment", a.id)],
			Answers: make([][]string, len(a.dates)),
		}

		if inner.Name == "" {
			inner.Name = "[unknown]"
		}

		for i := range a.dates {
			s := results[a.dates[i].ID]
			inner.Answers[i] = []string{appointmentColour(s), s}
			td.Points[i] += appointmentPoints(s)
		}

		td.Data = append(td.Data, inner)
	}

	bestPoints := math.Inf(-1)

	for i := range td.Points {
		if td.Points[i] > bestPoints {
			bestPoints = td.Points[i]
			td.Best = a.dates[i].Display
			td.BestNumber = i
		}
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := appointmentStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("date: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (a appointment) ValidateInput(data map[string][]string) error {
	if !a.NameRequired {
		return nil
	}
	if len(data[fmt.Sprintf("%s_name", a.id)]) == 0 {
		return fmt.Errorf("appointment: No name found")
	}
	if len(data[fmt.Sprintf("%s_name", a.id)][0]) == 0 {
		return fmt.Errorf("appointment: Name has zero length")
	}

	now := time.Now()
	for i := range a.dates {
		if len(data[a.dates[i].ID]) != 0 {
			if a.DisallowVotesInPast && a.dates[i].time.Before(now) {
				return fmt.Errorf("appointment: answer '%s' is in past (currently: %s)", a.dates[i].ID, now.Format(appointmentDateFormatWrite))
			}
			switch data[a.dates[i].ID][0] {
			case "✓", "👎", "X", "?":
				// Valid answer
			default:
				return fmt.Errorf("appointment: Unknown answer '%s'", data[a.dates[i].ID][0])
			}
		}
	}
	return nil
}

func (a appointment) GetDatabaseEntry(data map[string][]string) string {
	results := make(map[string]string)
	if len(data[fmt.Sprintf("%s_name", a.id)]) != 0 {
		results[fmt.Sprintf("%s_name", a.id)] = data[fmt.Sprintf("%s_name", a.id)][0]
	}
	if len(data[fmt.Sprintf("%s_comment", a.id)]) != 0 {
		results[fmt.Sprintf("%s_comment", a.id)] = data[fmt.Sprintf("%s_comment", a.id)][0]
	}
	for i := range a.dates {
		if len(data[a.dates[i].ID]) != 0 {
			results[a.dates[i].ID] = data[a.dates[i].ID][0]
		}
	}
	b, err := json.Marshal(results)
	if err != nil {
		return err.Error()
	}
	return string(b)
}
