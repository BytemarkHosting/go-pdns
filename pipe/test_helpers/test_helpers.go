package test_helpers

import(
	"fmt"
	"github.com/BytemarkHosting/go-pdns/pipe/backend"
	"testing"
)

func EmptyDispatch(b *backend.Backend, q *backend.Query)([]*backend.Response,error) {
	return nil, nil
}


func AssertEqualString(t *testing.T, a, b, msg string) {
	if a != b {
		t.Logf(fmt.Sprintf("%s:'\n%s\n'\nshould be the same as:'\n%s\n'", msg, a, b))
		t.FailNow()
	}
}

func AssertEqualInt(t *testing.T, a, b int, msg string) {
	if a != b {
		t.Logf(fmt.Sprintf("%s: %d should == %d", msg, a, b))
		t.FailNow()
	}
}

func RefuteError(t *testing.T, err error, msg string) {
	if err != nil {
		t.Logf("Error: %s was expected to be nil", msg, err)
		t.FailNow()
	}
}

func Assert(t *testing.T, condition bool, msg string) {
	if !condition {
		t.Log(msg)
		t.FailNow()
	}
}

func FakeQuery(protoVersion int) *backend.Query {
	if protoVersion < 1 || protoVersion > 3 {
		panic("Invalid protoVersion") 
	}

	q := backend.Query{
		ProtocolVersion: protoVersion,
		QClass: "IN",
		QType: "ANY",
		QName: "example.com",
		Id: "-1",
		RemoteIpAddress: "127.0.0.2",
	}
	if protoVersion > 1 {
		q.LocalIpAddress = "127.0.0.1"
	}
	if protoVersion > 2 {
		q.EdnsSubnetAddress = "127.0.0.3"
	}

	return &q
}

func FakeResponse(protoVersion int) *backend.Response {
	if protoVersion < 1 || protoVersion > 3 {
		panic("Invalid protoVersion") 
	}

	r := backend.Response{
		ProtocolVersion: protoVersion,
		QClass: "IN",
		QType: "ANY",
		QName: "example.com",
		Id: "-1",
		TTL: "3600",
		Content: "foo",
	}
	if protoVersion > 2 {
		r.ScopeBits = "24"
		r.Auth = "auth"
	}

	return &r
}

func FakeQueryString(t *testing.T, protoVersion int) string {
	str, err := FakeQuery(protoVersion).String()
	RefuteError(t, err, "Failed to serialise test query")
	return str
}

func FakeResponseString(t *testing.T, protoVersion int) string {
	str, err := FakeResponse(protoVersion).String()
	RefuteError(t, err, "Failed to serialise test response")
	return str
}

