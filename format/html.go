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
	"html/template"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
	"github.com/microcosm-cc/bluemonday"
)

// This needs a custom policy since we need to be more restrictive
// Currently, the only difference to helper.Sanitise* is not allowing id and class
var htmlPolicy *bluemonday.Policy

func init() {
	htmlPolicy = bluemonday.NewPolicy()
	htmlPolicy.AllowElements("a", "b", "br", "caption", "code", "del", "div", "em", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "i", "ins", "img", "kbd", "mark", "p", "pre", "q", "s", "samp", "strong", "sub", "sup", "u")
	htmlPolicy.AllowLists()
	htmlPolicy.AllowStandardURLs()
	htmlPolicy.AllowImages()
	htmlPolicy.AllowAttrs("href").OnElements("a")
	htmlPolicy.RequireNoReferrerOnLinks(true)
	htmlPolicy.AllowTables()
	htmlPolicy.AddTargetBlankToFullyQualifiedLinks(true)

	registry.RegisterFormatType(HTML{}, "html")
}

// HTML implements a format taking html source and transforming it into save html.
type HTML struct{}

// Format returns save html.
func (h HTML) Format(b []byte) template.HTML {
	return template.HTML(htmlPolicy.SanitizeBytes(b))
}

// FormatClean returns a save version, stripped of most formatting tags.
func (h HTML) FormatClean(b []byte) template.HTML {
	return helper.SanitiseByteClean(b)
}
