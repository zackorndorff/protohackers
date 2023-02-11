package main
import (
	//"fmt"
	"bytes"
	"testing"
)

var validtext = []string{
	"Hello, world!\n",
}
var invalidtext = []string{
	"\x7fHello, world!",
}

var invalidnames = []string{
	"/A\"z@wt)nVE~%$^(",
	"/m`p/pc",
	"/ZXml-+4wL",
	"/R4?N-lh1j",
	"/o~q\"BrK",
	"/kM=lxiak0k",
	"/~K~fKwku3CG/8J",
}

func TestValidText(t *testing.T) {
	for i, x := range validtext {
		if !textIsValid([]byte(x)) {
			t.Errorf("text %d ('%s') should be fine", i, x)
		}
	}
	for i, x := range invalidtext {
		if textIsValid([]byte(x)) {
			t.Errorf("text %d ('%s') should be invalid", i, x)
		}
	}
}

func TestInvalidNames(t *testing.T) {
	for i, name := range invalidnames {
		if !filenameIsIllegal(name) {
			t.Errorf("Filename %d ('%s') incorrectly valid", i, name)
		}
	}
}

func TestSmoke(t *testing.T) {
	repo := NewRepository()
	list, err := repo.List("/")
	if err != nil {
		t.Fatalf("error listing repo: %s", err)
	}

	if len(list) != 0 {
		t.Fatal("list of empty repo not empty")
	}

	contents := []byte("contents of file 1")
	rev, err := repo.Put("/file1", contents)
	if err != nil {
		t.Fatalf("error putting file: %s", err)
	}

	if rev != 1 {
		t.Fatalf("file was not put at revision 1, instead got %d", rev)
	}

	returned, err := repo.Get("/file1", false, 0)
	if err != nil {
		t.Fatalf("error retrieving file: %s", err)
	}
	if !bytes.Equal(contents, returned) {
		t.Fatalf("file contents wrong. Expected %s, got %s", contents, returned)
	}
}
