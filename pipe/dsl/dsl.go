// Copyright 2015 Bytemark Computer Consulting Ltd. All rights reserved
// Licensed under the GNU General Public License, version 2. See the LICENSE
// file for more details

// Simple DSL for pipebackend. Usage:
//
//	// Create new handle. You have to specify a default TTL here.
//	x := dsl.New()
//	root := regexp.QuoteMeta("example.com")
//
//	// most zones need SOA + NS records
//	x.SOA(root, func(c *dsl.Context) {
//		c.Reply("ns1.example.com hostmaster.example.com 1 3600 1800 86400 3600")
//	})
//
//	// Zones need NS records too. All replies will be returned
//	x.NS(root, func(c *dsl.Context) {
//		c.Reply("ns1.example.com")
//		c.Reply("ns2.example.com")
//		c.Reply("ns3.example.com")
//	})
//
//	// You don't have to use anonymous functions, of course
//	func answer(c *dsl.Context) {
//		switch c.Query.QType {
//			case "A"   : c.Reply("169.254.0.1")
//			case "AAAA": c.Reply("fe80::1"    )
//		}
//	}
//	x.A(root, answer)
//	x.AAAA(root, answer)
//
//	// Setting c.Error at any point will suppress *all* replies from being
//	// sent back. Instead, a FAIL response with the c.Error.Error() as the
//	// content is returned to powerdns
//	x.SSHFP(root, func(c *dsl.Context) {
//		c.Reply("1 1 f1d2d2f924e986ac86fdf7b36c94bcdf32beec15")
//		c.Error = errors.New("Don't use SSHFP on unsigned zones")
//		c.Reply("1 2 e242ed3bffccdf271b7fbaf34ed72d089537b42f")
//	})
//
//	// You can do anything in a callback, but be aware that powerdns has a
//	// time limit on responses and there is no request concurrency within a
//	// single pipe connection. pdns achieves concurrency through multiple
//	// backend connections instead
//	c := make(chan string)
//	x.MX(root, func(c *dsl.Context) {
//		c.Reply(<-c)
//	})
//
//	// If your regexp includes capture groups, they are quoted back to you.
//	// Here's a simple DNS echo server. Note the use of ReplyExtra to allow
//	// a non-default TTL to be set.
//	//
//	// Don't forget: DNS is supposed to be case-insensitive. Be careful.
//	c.TXT(`(.*)\.` + root, func(c *dsl.Context) {
//		c.ReplyTTL(c.Query.QName, c.Matches[0], 0)
//	})
//
//	// Dispatch is up to you. It will probably look like this, but you
//	// might want to add logging around the request or something more
//	// complicated (different DSL instance depending on backend version?)
//	func doit(b *backend.Backend, q *backend.Query) ([]*backend.Response, error) {
//		if q.QClass == "IN" {
//			return x.Lookup(q)
//		}
//		return nil, errors.New("Only IN QClass is supported")
//	}
//
//	pipe := backend.New( r, w, "Example backend" )
//	err1 := pipe.Negotiate() // do check for errors
//	err2 := pipe.Run(doit)
//
//
//
package dsl

import (
	"github.com/BytemarkHosting/go-pdns/pipe/backend"
	"regexp"
)

// Instances of this struct are used to hold onto registered callbacks, etc.
type DSL struct {
	callbacks  map[string][]callbackNode
	qtypeSort  []string
	defaultTTL int

	beforeCallback Callback
}

// Get a new builder with a default TTL of one hour
func New() *DSL {
	return NewWithTTL(3600)
}

// Get a new builder, specifying a default TTL explicitly
func NewWithTTL(ttl int) *DSL {
	return &DSL{
		callbacks:  make(map[string][]callbackNode),
		qtypeSort:  make([]string, 0),
		defaultTTL: ttl,
	}
}

// Callbacks are registered against the DSL instance and run against incoming
// queries if the regexp they are registered with matches the QName of the query
type Callback func(c *Context)

type callbackNode struct {
	matcher *regexp.Regexp
	fn      Callback
}

// Register a callback to run before every request. Set c.Error to halt
// processing, or mutate the context however you like.
func (d *DSL) Before(f Callback) {
	d.beforeCallback = f
}

// Register a callback to be run whenever a query with a QName matching the
// regular expression comes in. The regex is provided as a string (matcher)
// to keep ordinary invocations short; it's compiled immediately with
// regexp.MustCompile. Don't forget to anchor your regexes!
//
// If match groups are included in the regex, then any matched text is placed in
// the Context the callback receives.
//
// Callbacks are run with slightly obtuse ordering: all callbacks of a qtype
// are run in the order they were registered. We iterate the list of qtypes
// in the order that a callback with a matching qtype was *first* registered.
// If your pdns server has the "noshuffle" configuration directive, the order
// will be reflected in the responses returned by it; future concurrent DSL
// should maintain this ordering.
func (d *DSL) Register(qtype string, re *regexp.Regexp, f Callback) {

	// Maintain our obtuse sense of order
	alreadyIn := false
	for _, prospect := range d.qtypeSort {
		if prospect == qtype {
			alreadyIn = true
			break
		}
	}
	if !alreadyIn {
		d.qtypeSort = append(d.qtypeSort, qtype)
	}

	node := callbackNode{matcher: re, fn: f}
	d.callbacks[qtype] = append(d.callbacks[qtype], node)
}

// Once we're concurrent, this method will create the context and return it
func (d *DSL) runNode(c *Context, node *callbackNode) {
	matches := node.matcher.FindStringSubmatch(c.Query.QName)

	if matches != nil && len(matches) > 0 {
		// Probably unnecessary, but ensure that the previous value of
		// Matches is preserved. This could also be = nil
		oldmatches := c.Matches
		defer func(c *Context) { c.Matches = oldmatches }(c)

		// The first match is the whole thing, followed by the capture
		// groups. We're only interested in the latter.
		c.Matches = matches[1:]

		if d.beforeCallback != nil {
			d.beforeCallback(c)
		}

		if c.Error == nil {
			node.fn(c)
		}
	}
}

// Run all registered callbacks against the query. If any callbacks report an
// error, we halt and return the error only (partially constructed responses are
// discarded).
//
// For now, callbacks are run sequentially, rather than in parallel. There could
// be a speedup to running each callback in its own goroutine. Currently, all
// callbacks share the same context instance; we'd have to change that if we
// ran them in parallel.
func (d *DSL) Lookup(q *backend.Query) ([]*backend.Response, error) {
	c := Context{
		DefaultTTL: d.defaultTTL,
		Query:      q,
		Answers:    make([]*backend.Response, 0),
		Error:      nil,
	}

	var runOn []string
	if q.QType == "ANY" {
		runOn = d.qtypeSort
	} else {
		runOn = []string{q.QType}
	}

	for _, qtype := range runOn {
		c.QType = qtype
		for _, node := range d.callbacks[qtype] {
			d.runNode(&c, &node)
			if c.Error != nil {
				return nil, c.Error
			}
		}
	}

	return c.Answers, nil
}

// Reports the registered callbacks, in order. Handy for testing or status.
func (d *DSL) String() string {
	out := ""
	for _, qtype := range d.qtypeSort {
		out = out + qtype + "\t:"
		for _, node := range d.callbacks[qtype] {
			out = out + "\t" + node.matcher.String() + "\n"
		}
	}
	return out
}
