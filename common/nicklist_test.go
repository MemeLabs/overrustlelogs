package common

import (
	"log"
	"testing"
)

func TestWrite(t *testing.T) {
	n := NickList{}
	n.Add("foo")
	n.Add("bar")
	n.Add("baz")
	n.Add("qux")
	if err := n.WriteTo("/tmp/nicks"); err != nil {
		log.Printf("error saving nicks list %s", err)
		t.Fail()
	}
}

func TestRead(t *testing.T) {
	n := NickList{}
	n.Add("foo")
	n.Add("bar")
	n.Add("baz")
	n.Add("qux")
	if err := n.WriteTo("/tmp/nicks"); err != nil {
		log.Printf("error writing nick list %s", err)
		t.Fail()
		return
	}

	r := NickList{}
	if err := r.ReadFrom("/tmp/nicks"); err != nil {
		log.Printf("error reading nick list %s", err)
		t.Fail()
		return
	}

	for _, k := range []string{"foo", "bar", "baz", "qux"} {
		if _, ok := r[k]; !ok {
			log.Printf("nick not found %s", k)
			t.Fail()
			return
		}
	}
}
