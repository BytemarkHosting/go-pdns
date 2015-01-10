// Copyright 2015 Bytemark Computer Consulting Ltd. All rights reserved
// Licensed under the GNU General Public License, version 2. See the LICENSE
// file for more details

// Handler for the PowerDNS pipebackend protocol, as documented here:
// https://doc.powerdns.com/md/authoritative/backend-pipe/
//
// Can speak all three (at time of writing) protocol versions.
//
// Usage:
//
//	backend := backend.New(

package backend

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Backend struct {
	// The text the backend will serve to a successful hello message
	Banner string

	// The protocol version negotiated with the remote end
	ProtocolVersion int

	io *bufio.ReadWriter
}

// A callback of this type is executed whenever a query is received. If an error
// is returned, the responses are ignored and the error text is returned to the
// backend. Otherwise, the responses are serialised and sent back in order.
type Callback func(b *Backend, q *Query) ([]*Response, error)

// Build a new backend object. The banner is reported to the client upon
// successful negotiation; the io can be anything.
func New(r io.Reader, w io.Writer, banner string) *Backend {
	io := bufio.NewReadWriter(
		bufio.NewReader(r),
		bufio.NewWriter(w),
	)
	return &Backend{Banner: banner, io: io}
}

// Does initial handshake with peer. Returns nil, or an error
// Note that the pipebackend protocol documentation states that if negotiation
// fails, the process should retry, not exit itself.
func (b *Backend) Negotiate() error {
	hello, err := b.io.ReadString('\n')
	if err != nil {
		return err
	}

	// We're not interested in the trailing newlines
	parts := strings.Split(strings.TrimRight(hello, "\r\n"), "\t")

	if len(parts) != 2 || parts[0] != "HELO" {
		return errors.New("Bad hello from client")
	}

	version, err := strconv.Atoi(parts[1])
	if version < 1 || version > 3 || err != nil {
		return errors.New("Unknown protocol version requested")
	}

	_, err = b.io.WriteString(fmt.Sprintf("OK\t%s\n", b.Banner))
	if err == nil {
		err = b.io.Flush()
	}

	if err == nil {
		b.ProtocolVersion = version
	}
	return err
}

func (b *Backend) handleQ(data string, callback Callback) ([]*Response, error) {
	query := Query{ProtocolVersion: b.ProtocolVersion}

	err := query.fromData(data)
	if err != nil {
		return nil, err
	}

	return callback(b, &query)
}

// TODO
func (b *Backend) handleAXFR() ([]*Response, error) {
	return nil, errors.New("AXFR requests not supported")
}

// Reads lines in a loop, processing them by executing the provided callback
// and writing appropriate output in response, sequentially, until we hit an
// error or our IO hits EOF
func (b *Backend) Run(callback Callback) error {
	responses := make([]*Response, 0)

	for {
		line, err := b.io.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		parts := strings.SplitN(strings.TrimRight(line, "\n"), "\t", 2)
		if len(parts) == 2 {

		}

		switch parts[0] {
		case "Q":
			responses, err = b.handleQ(parts[1], callback)
		case "PING":
			responses, err = nil, nil // We just need to return END
		case "AXFR":
			responses, err = b.handleAXFR()
		default:
			responses, err = nil, errors.New("Bad command")
		}

		if err != nil {
			// avoid protocol errors
			clean := strings.Replace(err.Error(), "\n", " ", -1)
			msg := fmt.Sprintf("LOG\tError handling line: %s\nFAIL\n", clean)
			_, err := b.io.WriteString(msg)
			if err != nil {
				return fmt.Errorf("%s while writing FAIL response", err)
			}
			//
			err = b.io.Flush()
			if err != nil {
				return fmt.Errorf("%s while flushing FAIL response", err)
			}
			continue
		}

		// DATA (if there are any records to return)
		for _, response := range responses {
			// Always output a line of the right protocol version
			// TODO: panic if it's set to a wrong non-zero value?
			response.ProtocolVersion = b.ProtocolVersion
			data, err := response.String()
			if err != nil {
				data = "LOG\tError serialising response: " + err.Error() + "\n"
			}
			_, err = b.io.WriteString(data)
			if err != nil {
				return fmt.Errorf("%s while writing DATA response", err)
			}
		}

		// END
		_, err = b.io.WriteString("END\n")
		if err == nil {
			err = b.io.Flush()
		}

		if err != nil {
			return fmt.Errorf("%s while writing END", err)
		}
	}

	// We should never hit this at the moment.
	// TODO: graceful signal handling - intercept and ensure current query
	// completes before breaking the above loop and returning nil here
	return nil
}
