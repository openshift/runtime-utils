// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"strings"
)

// tagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, tagOptions) {
<<<<<<< HEAD
	tag, opt, _ := strings.Cut(tag, ",")
	return tag, tagOptions(opt)
=======
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tagOptions(tag[idx+1:])
	}
	return tag, tagOptions("")
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.
func (o tagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
<<<<<<< HEAD
		var name string
		name, s, _ = strings.Cut(s, ",")
		if name == optionName {
			return true
		}
=======
		var next string
		i := strings.Index(s, ",")
		if i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == optionName {
			return true
		}
		s = next
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
	}
	return false
}
