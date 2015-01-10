package backend_test

import (
	h "../test_helpers"
	"bytes"
	"errors"
	"fmt"
	. "github.com/BytemarkHosting/go-pdns/pipe/backend"
	"strings"
	"testing"
)

// Test serializing Query & Response instances - we use them in the tests
func TestQueryStringV1(t *testing.T) {
	h.AssertEqualString(
		t, "Q\texample.com\tIN\tANY\t-1\t127.0.0.2\n",
		h.FakeQueryString(t, 1), "V1 query serialisation problem",
	)
}

func TestQueryStringV2(t *testing.T) {
	h.AssertEqualString(
		t, "Q\texample.com\tIN\tANY\t-1\t127.0.0.2\t127.0.0.1\n",
		h.FakeQueryString(t, 2), "V2 query serialisation problem",
	)
}

func TestQueryStringV3(t *testing.T) {
	h.AssertEqualString(
		t, "Q\texample.com\tIN\tANY\t-1\t127.0.0.2\t127.0.0.1\t127.0.0.3\n",
		h.FakeQueryString(t, 3), "V3 query serialisation problem",
	)
}

// Ensure we serialize Response instances correctly - we use them in the tests
func TestResponseStringV1andV2(t *testing.T) {
	exemplar := "DATA\texample.com\tIN\tANY\t3600\t-1\tfoo\n"

	h.AssertEqualString(t, exemplar, h.FakeResponseString(t, 1), "V1 response serialisation problem")
	h.AssertEqualString(t, exemplar, h.FakeResponseString(t, 2), "V2 response serialisation problem")
}

func TestResponseStringV3(t *testing.T) {
	h.AssertEqualString(
		t, "DATA\t24\tauth\texample.com\tIN\tANY\t3600\t-1\tfoo\n",
		h.FakeResponseString(t, 3), "V3 response serialisation problem",
	)
}

func BuildAndNegotiate(t *testing.T, protoVersion int) (*Backend, *bytes.Buffer, *bytes.Buffer) {
	r := bytes.NewBufferString(fmt.Sprintf("HELO\t%d\n", protoVersion))
	w := &bytes.Buffer{}
	b := New(r, w, "Testing Backend")

	h.RefuteError(t, b.Negotiate(), "Negotiation failed")
	h.AssertEqualInt(t, protoVersion, b.ProtocolVersion, "Bad protocol version")
	h.AssertEqualString(t, "OK\tTesting Backend\n", w.String(), "Bad response to HELO")
	w.Reset()

	return b, r, w
}

func AssertRun(t *testing.T, b *Backend, f Callback) {
	err := b.Run(f)
	h.RefuteError(t, err, "Running backend")
}

func AssertProtocolVersionNegotiation(t *testing.T, protoVersion int) {
	b, r, w := BuildAndNegotiate(t, protoVersion)
	r.WriteString(h.FakeQueryString(t, protoVersion))

	// We also test that the negotiated version can be handled
	AssertRun(t, b, h.EmptyDispatch)
	h.AssertEqualString(t, "END\n", w.String(), "Unexpected response")
	w.Reset()

	exp := fmt.Sprintf(
		"LOG\tError handling line: v%d query should have %d data parts\nFAIL\n",
		protoVersion, protoVersion+4,
	)

	// Test a short Q
	r.WriteString("Q\tfoo\n")
	AssertRun(t, b, h.EmptyDispatch)
	h.AssertEqualString(t, exp, w.String(), "Unexpected response")
	w.Reset()

	// test a long Q
	long := strings.TrimRight(h.FakeQueryString(t, protoVersion), "\n") + "\tfoo\n"
	r.WriteString(long)
	AssertRun(t, b, h.EmptyDispatch)
	h.AssertEqualString(t, exp, w.String(), "Unexpected response")
}

func TestNegotiatedVersion1(t *testing.T) {
	AssertProtocolVersionNegotiation(t, 1)
}
func TestNegotiatedVersion2(t *testing.T) {
	AssertProtocolVersionNegotiation(t, 2)
}

func TestNegotiatedVersion3(t *testing.T) {
	AssertProtocolVersionNegotiation(t, 3)
}

func TestQueriesArePassedToDispatcher(t *testing.T) {
	b, r, w := BuildAndNegotiate(t, 3)
	var outQ *Query
	runs := 0

	expected := h.FakeQueryString(t, 3)
	r.WriteString(expected)

	AssertRun(t, b, func(b *Backend, q *Query) ([]*Response, error) {
		runs = runs + 1
		outQ = q
		return nil, nil
	})
	h.AssertEqualInt(t, 1, runs, "Exactly one dispatch callback expected")

	txt, _ := outQ.String()
	h.AssertEqualString(t, expected, txt, "Wrong query dispatched")
	h.AssertEqualString(t, "END\n", w.String(), "Unexpected response")
}

func TestResponsesFromDispatcherArePassedToAsker(t *testing.T) {
	b, r, w := BuildAndNegotiate(t, 3)
	r.WriteString(h.FakeQueryString(t, 3))

	fr := h.FakeResponse(3)
	AssertRun(t, b, func(b *Backend, q *Query) ([]*Response, error) {
		return []*Response{fr, fr}, nil
	})

	exp := fmt.Sprintf("%s%sEND\n", h.FakeResponseString(t, 3), h.FakeResponseString(t, 3))
	h.AssertEqualString(t, exp, w.String(), "Bad response")
}

func TestErrorFromDispatcherSuppressesResponses(t *testing.T) {
	b, r, w := BuildAndNegotiate(t, 3)
	r.WriteString(h.FakeQueryString(t, 3))

	fr := h.FakeResponse(3)
	AssertRun(t, b, func(b *Backend, q *Query) ([]*Response, error) {
		return []*Response{fr, fr}, errors.New("Test\nerror")
	})
	h.AssertEqualString(t, "LOG\tError handling line: Test error\nFAIL\n", w.String(), "Bad response")
}

func TestHandlesPing(t *testing.T) {
	b, r, w := BuildAndNegotiate(t, 3)
	r.WriteString("PING\n")
	AssertRun(t, b, h.EmptyDispatch)
	h.AssertEqualString(t, "END\n", w.String(), "Bad response")
}

func TestAXFRIsTODO(t *testing.T) {
	b, r, w := BuildAndNegotiate(t, 3)
	r.WriteString("AXFR\n")
	AssertRun(t, b, h.EmptyDispatch)
	h.AssertEqualString(t, "LOG\tError handling line: AXFR requests not supported\nFAIL\n", w.String(), "Bad response")
}

func TestUnknownCommand(t *testing.T) {
	b, r, w := BuildAndNegotiate(t, 3)
	r.WriteString("GOGOGO\n")
	AssertRun(t, b, h.EmptyDispatch)
	h.AssertEqualString(t, "LOG\tError handling line: Bad command\nFAIL\n", w.String(), "Bad response")
}
