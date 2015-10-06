package rpc

import (
	"bytes"
	"encoding/base64"
	"github.com/ugorji/go/codec"
	"io/ioutil"
	"testing"
)

// Test an output from objective C that was breaking the server
func TestObjcOutput(t *testing.T) {
	dat, err := ioutil.ReadFile("objc_output.dat")
	if err != nil {
		t.Fatal(err)
	}
	v, err := base64.StdEncoding.DecodeString(string(dat))
	if err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer(v)
	var i int
	mh := codec.MsgpackHandle{WriteExt: true}
	dec := codec.NewDecoder(buf, &mh)
	err = dec.Decode(&i)
	if err != nil {
		t.Fatal(err)
	}

	if i != buf.Len() {
		t.Fatalf("Bad frame: %d != %d", i, buf.Len())
	}

	var a interface{}
	err = dec.Decode(&a)
	if err != nil {
		t.Fatal(err)
	}
}
