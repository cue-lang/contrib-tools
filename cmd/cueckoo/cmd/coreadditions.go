// Copyright 2021 The CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// ****************************************************************
// This file contains generally useful additions to the core package
// ****************************************************************

package cmd

import "fmt"

func errcheck(err error) {
	if err != nil {
		panic(panicError{
			Err: err,
		})
	}
}

func check(err error, format string, args ...interface{}) {
	if err != nil {
		raise(format, args...)
	}
}

func raise(format string, args ...interface{}) {
	panic(panicError{
		Err: fmt.Errorf(format, args...),
	})
}
