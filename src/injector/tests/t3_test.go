package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestValidateTestName(t *testing.T) {
	testCases := []struct {
		testName  string
		tagString string
	}{
		{
			"a test case with no tags",
			"",
		},
		{
			"a test case with valid tags [LinuxOnly] [NodeConformance] [Serial] [Disruptive]",
			"",
		},
		{
			"a flaky test case that is invalid [Flaky]",
			"[Flaky]",
		},
		{
			"a feature test case that is invalid [Feature:Awesome]",
			"[Feature:Awesome]",
		},
		{
			"an alpha test case that is invalid [Alpha]",
			"[Alpha]",
		},
		{
			"a test case with multiple invalid tags [Flaky] [Feature:Awesome] [Alpha]",
			"[Flaky],[Feature:Awesome],[Alpha]",
		},
		{
			"[sig-awesome] [Alpha] [Disruptive] a test case with valid and invalid tags [Serial] [Flaky]",
			"[Alpha],[Flaky]",
		},
	}
	for i, tc := range testCases {
		err := validateTestName(tc.testName)
		if err != nil {
			if tc.tagString == "" {
				t.Errorf("test case[%d]: expected no validate error, got %q", i, err.Error())
			} else {
				expectedMsg := fmt.Sprintf("'%s' cannot have invalid tags %s", tc.testName, tc.tagString)
				actualMsg := err.Error()
				if actualMsg != expectedMsg {
					t.Errorf("test case[%d]: expected error message %q, got %q", i, expectedMsg, actualMsg)
				}
			}
		}
	}
}