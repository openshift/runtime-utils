// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build gofuzz
<<<<<<< HEAD
=======
// +build gofuzz
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)

package json

import (
	"fmt"
)

func Fuzz(data []byte) (score int) {
<<<<<<< HEAD
	for _, ctor := range []func() any{
		func() any { return new(any) },
		func() any { return new(map[string]any) },
		func() any { return new([]any) },
=======
	for _, ctor := range []func() interface{}{
		func() interface{} { return new(interface{}) },
		func() interface{} { return new(map[string]interface{}) },
		func() interface{} { return new([]interface{}) },
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
	} {
		v := ctor()
		err := Unmarshal(data, v)
		if err != nil {
			continue
		}
		score = 1

		m, err := Marshal(v)
		if err != nil {
			fmt.Printf("v=%#v\n", v)
			panic(err)
		}

		u := ctor()
		err = Unmarshal(m, u)
		if err != nil {
			fmt.Printf("v=%#v\n", v)
			fmt.Printf("m=%s\n", m)
			panic(err)
		}
	}

	return
}
