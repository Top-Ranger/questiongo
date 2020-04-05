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

package format

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

func init() {
	err := registry.RegisterFormatType(Markdown{}, "markdown")
	if err != nil {
		panic(err)
	}
}

// Markdown takes input in the markdown format (including some extensions) and returns save HTML.
type Markdown struct{}

// Format returns a save html version of the Markdown input.
func (m Markdown) Format(b []byte) template.HTML {
	buf := bytes.NewBuffer(make([]byte, 0, len(b)*2))
	md := goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithRendererOptions(html.WithHardWraps()))
	err := md.Convert(b, buf)
	if err != nil {
		return template.HTML(helper.SanitiseString(fmt.Sprintf("Error rendering markdown: %s", err.Error())))
	}

	return helper.SanitiseByte(buf.Bytes())
}

// FormatClean returns a save html version of the Markdown input. Most formatting is stripped from the output.
func (m Markdown) FormatClean(b []byte) template.HTML {
	buf := bytes.NewBuffer(make([]byte, 0, len(b)*2))
	md := goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithRendererOptions(html.WithHardWraps()))
	err := md.Convert(b, buf)
	if err != nil {
		return template.HTML(helper.SanitiseString(fmt.Sprintf("Error rendering markdown: %s", err.Error())))
	}

	return helper.SanitiseByteClean(buf.Bytes())
}
