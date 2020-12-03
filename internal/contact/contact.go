package contact

import (
	"strings"
	"strconv"
	"github.com/pkg/errors"
)

var errIdZero = errors.New("id: zero")
var errIdMissing = errors.New("id: unspecified")
var errIdentifier = errors.New("id: invalid")
var errIdNotInteger = errors.New("id: !integer")

// ContactObjectNode parse optional <oid[@app]> service node-id definition
func ContactObjectNode(contact string) (oid int64, host string, err error) {
	// contains pid[@host] declaration ?
	
	contact, host = ContactServiceNode(contact)

	oid, err = strconv.ParseInt(contact, 10, 64)
	
	if err != nil {
		err = errIdNotInteger
	}

	if oid == 0 {
		err = errIdZero
	}

	return oid, host, err
}

// ContactServiceNode parses optional <oid[@app]> service app-node-id definition
func ContactServiceNode(contact string) (oid, host string) {

	oid = strings.TrimSpace(contact)

	if at := strings.LastIndexByte(oid, '@'); at > 0 {
		host, oid = oid[at+1:], oid[:at]
	}

	return oid, host
}

// NodeServiceContact prints optional <oid[@app]> service node-id definition
func NodeServiceContact(oid, host string) (contact string, err error) {

	oid = strings.TrimSpace(oid)
	host = strings.TrimSpace(host)

	if host == "" {
		return oid, nil
	}

	// require: object.id
	if oid == "" {
		return "", nil // errIdMissing
	}

	// default
	contact = oid

	// valid hostname &
	for _, c := range host {
		switch {
		case '0' <= c && c <= '9': // Digits
		case 'A' <= c && c <= 'Z': // ASCII Upper
		case 'a' <= c && c <= 'z': // ASCII Lower
		case '-' == c || c == '_': // Extra Delims
		case '[' == c || c == ']': // IPv6 Address
		case '.' == c || c == ':': // IPv4 & :PORT 
		default:
			// invalid name syntax
			return "", errIdentifier
		}
	}
	// optional: @service-node-id
	if host != "" {
		contact += "@" + host
	}

	return contact, nil
}

func NodeObjectContact(oid int64, host string) (contact string, err error) {
	// require: [o]bject[id]entity
	if oid == 0 {
		return "", errIdMissing
	}
	// require: default
	contact = strconv.FormatInt(oid, 10)
	// optional: @node
	contact, err = NodeServiceContact(contact, host)

	return contact, err
}