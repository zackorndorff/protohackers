package main

import (
	//"encoding/binary"

	"strings"
	"testing"
)

var testAddresses = []string{
	"7DVMhl72SdNwS9pNiV0fryIIPg",
	"7YWHMfk9JZe0LM0g1ZauHuiSxhI",
	"7F1u3wSD5RbOHQmupo9nx4TnhQ",
	"7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX",
	"7LOrwbDlS8NujgjddyogWgIM93MV5N2VR",
	"7adNeSwJkMakpEcln9HEtthSRtxdmEHOT8T",
}

func TestRegex(t *testing.T) {
	for i, vec := range testAddresses {
		if coinRe.FindString(vec) == "" {
			t.Errorf("Failed a test vector (%d): %s", i, vec)
		}
	}
}

var notAddresses = []string{
	"7aKj5oAZ0TEOmOXOhErqj2T5uMDF64xvvi-w22cNT6CkBsNdn8gdy0lOFDUyBHUM-1234",
	"75wFxe5TfnkcS3RIGTHABheno6gwnA0asYt",
	"785vE6mDLTTcrQDqR9DSLdcSR5",
	"uBJAKsN5eTrMxOQzIpgR1IMI6h5",
}

func TestRegexNegative(t *testing.T) {
	for i, vec := range notAddresses {
		res := coinRe.FindString(vec)
		if res != "" {
			t.Errorf("Failed a test vector (%d): %s (got %s)", i, vec, res)
		}
	}
}

func subInAddress(vector []string, addr string) []string {
	res := []string{}
	for _, i := range vector {
		res = append(res, strings.ReplaceAll(i, "%s", addr))
	}
	return res
}

func TestMangle(t *testing.T) {
	addr := "foobarbaz"
	testFmtStrs := []string{
		"%s is my address",
		"My address is %s",
		"Please send the payment of 750 Boguscoins to %s",
		"My address is %s , please pay ASAP to %s",
		"Two next to each other works: %s %s",
	}
	correctAnswers := subInAddress(testFmtStrs, addr)
	testVectors := [][]string{}
	for _, addr := range testAddresses {
		testVectors = append(testVectors, subInAddress(testFmtStrs, addr))
	}
	for i, vec := range testVectors {
		if len(testFmtStrs) != len(vec) {
			t.Fatalf("Vector %d is the wrong length.", i)
		}
		for j := 0; j < len(testFmtStrs); j++ {
			mangled := mangleMessage(vec[j], addr)
			if mangled != correctAnswers[j] {
				t.Errorf("Failed test vector %d.%d: [%s] %s", i, j, correctAnswers[j], vec[j])
			}
		}
	}

}
