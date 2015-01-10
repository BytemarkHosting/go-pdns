package backend

import (
	"errors"
	"fmt"
	"strings"
)

// Represents a query received by the backend. Certain fields may be blank,
// depending on negotiated protocol version - which is stored in ProtocolVersion
//
// QName, QClass, QType, Id and RemoteIpAddress are present in all versions
// LocalIpAddress was added in version 2
// EdnsSubnetAddress was added in version 3
type Query struct {
	ProtocolVersion   int
	QName             string
	QClass            string
	QType             string
	Id                string
	RemoteIpAddress   string
	LocalIpAddress    string
	EdnsSubnetAddress string
}

func (q *Query) fromData(data string) (err error) {
	parts := strings.Split(data, "\t")

	// ugh
	switch q.ProtocolVersion {
	case 1:
		if len(parts) != 5 {
			return errors.New("v1 query should have 5 data parts")
		}
	case 2:
		if len(parts) != 6 {
			return errors.New("v2 query should have 6 data parts")
		}
		q.LocalIpAddress = parts[5]
	case 3:
		if len(parts) != 7 {
			return errors.New("v3 query should have 7 data parts")
		}
		q.LocalIpAddress = parts[5]
		q.EdnsSubnetAddress = parts[6]
	default:
		return errors.New("Unknown protocol version in query")
	}

	// common
	q.QName = parts[0]
	q.QClass = parts[1]
	q.QType = parts[2]
	q.Id = parts[3]
	q.RemoteIpAddress = parts[4]

	return nil
}

func (q *Query) String() (string, error) {
	if q.ProtocolVersion < 1 || q.ProtocolVersion > 3 {
		return "", errors.New("Unknown protocol version in query")
	}
	switch q.ProtocolVersion {
	case 1:
		return fmt.Sprintf(
			"Q\t%s\t%s\t%s\t%s\t%s\n",
			q.QName, q.QClass, q.QType, q.Id, q.RemoteIpAddress,
		), nil
	case 2:
		return fmt.Sprintf(
			"Q\t%s\t%s\t%s\t%s\t%s\t%s\n",
			q.QName, q.QClass, q.QType, q.Id, q.RemoteIpAddress,
			q.LocalIpAddress,
		), nil
	case 3:
		return fmt.Sprintf(
			"Q\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			q.QName, q.QClass, q.QType, q.Id, q.RemoteIpAddress,
			q.LocalIpAddress, q.EdnsSubnetAddress,
		), nil
	}

	return "", errors.New("Unknown protocol version in query")
}
