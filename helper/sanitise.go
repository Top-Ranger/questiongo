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

package helper

import (
	"html/template"
	"io"

	"github.com/microcosm-cc/bluemonday"
)

var defaultPolicy *bluemonday.Policy
var cleanPolicy *bluemonday.Policy

func init() {
	cleanPolicy = bluemonday.NewPolicy()
	cleanPolicy.AllowElements("img", "abbr")
	cleanPolicy.AllowAttrs("title").OnElements("abbr")
	cleanPolicy.AllowStandardURLs()
	cleanPolicy.AllowImages()
	defaultPolicy = bluemonday.NewPolicy()
	defaultPolicy.AllowElements("a", "b", "blockquote", "br", "caption", "code", "del", "div", "em", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "i", "ins", "img", "kbd", "mark", "p", "pre", "q", "s", "samp", "strong", "sub", "sup", "u", "abbr")
	defaultPolicy.AllowLists()
	defaultPolicy.AllowStandardURLs()
	defaultPolicy.AllowImages()
	defaultPolicy.AllowAttrs("id", "class", "hidden").Globally()
	defaultPolicy.AllowAttrs("href").OnElements("a")
	defaultPolicy.AllowAttrs("title").OnElements("abbr")
	defaultPolicy.RequireNoReferrerOnLinks(true)
	defaultPolicy.AllowTables()
	defaultPolicy.AddTargetBlankToFullyQualifiedLinks(true)
}

// SanitiseReader returns a save HTML version of the content provided by the reader.
func SanitiseReader(r io.Reader) template.HTML {
	b := defaultPolicy.SanitizeReader(r)
	return template.HTML(b.String())
}

// SanitiseString returns a save HTML version of the content provided.
func SanitiseString(s string) template.HTML {
	return template.HTML(defaultPolicy.Sanitize(s))
}

// SanitiseByte returns a save HTML version of the content provided.
func SanitiseByte(b []byte) template.HTML {
	return template.HTML(defaultPolicy.SanitizeBytes(b))
}

// SanitiseReaderClean returns a save HTML version of the content provided by the reader.
// Most formatting options are stripped.
func SanitiseReaderClean(r io.Reader) template.HTML {
	b := cleanPolicy.SanitizeReader(r)
	return template.HTML(b.String())
}

// SanitiseStringClean returns a save HTML version of the content provided.
// Most formatting options are stripped.
func SanitiseStringClean(s string) template.HTML {
	return template.HTML(cleanPolicy.Sanitize(s))
}

// SanitiseByteClean returns a save HTML version of the content provided.
// Most formatting options are stripped.
func SanitiseByteClean(b []byte) template.HTML {
	return template.HTML(cleanPolicy.SanitizeBytes(b))
}
