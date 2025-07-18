package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCommentToConformanceData(t *testing.T) {
	tcs := []struct {
		desc     string
		input    string
		expected *ConformanceData
	}{
		{
			desc: "Empty comment leads to nil",
		}, {
			desc:  "No Release or Testname leads to nil",
			input: "Description: foo",
		}, {
			desc:  "Release but no Testname should result in nil",
			input: "Release: v1.1\nDescription: foo",
		}, {
			desc:     "Testname but no Release does not result in nil",
			input:    "Testname: mytest\nDescription: foo",
			expected: &ConformanceData{TestName: "mytest", Description: "foo"},
		}, {
			desc:     "All fields parsed and newlines and whitespace removed from description",
			input:    "Release: v1.1\n\t\tTestname: mytest\n\t\tDescription: foo\n\t\tbar\ndone",
			expected: &ConformanceData{TestName: "mytest", Release: "v1.1", Description: "foo bar done"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			out := commentToConformanceData(tc.input)
			if !reflect.DeepEqual(out, tc.expected) {
				t.Errorf("Expected %#v but got %#v", tc.expected, out)
			}
		})
	}
}