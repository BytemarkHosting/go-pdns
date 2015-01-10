package dsl

import (
	"github.com/BytemarkHosting/go-pdns/pipe/backend"
	"strconv"
)

// Callbacks are run with a context instance, which allows them to accumulate
// answers while maintaining a short type signature. It will also make
// concurrent callbacks easier, when we handle that, but for now one context
// is maintained across all callbacks for a particular query
type Context struct {
	// Replies that don't specify a TTL will be given this instead.
	DefaultTTL int

	// The query that triggered this callback run. Note that its QType
	// member may be "ANY"
	Query *backend.Query

	// QType this callback is being run as. Matches the qtype field given
	// with the callback at the time DSL.Register was called
	QType string

	// The callback is registered with a regexp; if that regexp contains
	// any match groups, then the matched text is placed here.
	Matches []string

	// Set this if an error has been encountered; no more callbacks will be
	// run, and the error text (only) will be reported to the backend.
	Error error

	// Answers to be sent to the backend are stored here. Context.Reply()
	// calls, etc, generate answers and put them here, for instance.
	// If multiple callbacks are being run, then later callbacks will be
	// able to see the answers earlier ones generated (for now)
	Answers []*backend.Response
}

// Add an answer, using default QName and TTL for the query
func (c *Context) Reply(content string) {
	c.ReplyExtra(c.Query.QName, content, c.DefaultTTL)
}

// Add an answer, using the default QName but specifying a particular TTL
func (c *Context) ReplyTTL(content string, ttl int) {
	c.ReplyExtra(c.Query.QName, content, ttl)
}

// Add an answer, specifying both QName and TTL.
func (c *Context) ReplyExtra(qname, content string, ttl int) {
	c.Answers = append(c.Answers, &backend.Response{
		QName:   qname,
		QClass:  c.Query.QClass,
		QType:   c.QType, // q.Query.QType may == "ANY"
		Id:      c.Query.Id,
		Content: content,
		TTL:     strconv.Itoa(ttl),
	})
}
