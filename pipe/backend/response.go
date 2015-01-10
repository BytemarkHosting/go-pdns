package backend

import (
	"errors"
	"fmt"
)

// A response to be sent back in answer to a query. Again, some fields may be
// blank, depending on protocol version.
//
// QName, QClass, QType, TTL, Id and Content are present in all versions
// No additions in version 2
// ScopeBits and Auth were added in version 3
type Response struct {
	ProtocolVersion int
	ScopeBits       string
	Auth            string
	QName           string
	QClass          string
	QType           string
	TTL             string
	Id              string
	Content         string
}

// Gives the response in a serialized form suitable for squirting on the wire
func (r *Response) String() (string, error) {
	switch r.ProtocolVersion {
	case 1, 2:
		return fmt.Sprintf(
			"DATA\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.QName, r.QClass, r.QType, r.TTL, r.Id, r.Content,
		), nil
	case 3:
		return fmt.Sprintf(
			"DATA\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.ScopeBits, r.Auth, r.QName, r.QClass, r.QType, r.TTL, r.Id, r.Content,
		), nil
	}
	return "", errors.New("Unknown protocol version in response")
}
