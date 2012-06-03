// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Extended and bugfixes by Miek Gieben. Copyright 2010-2012.

// DOMAIN NAME SYSTEM
//
// Package dns implements a full featured interface to the Domain Name System.
// Server- and client-side programming is supported.
// The package allows complete control over what is send out to the DNS. The package
// API follows the less-is-more principle, by presenting a small, clean interface.
//
// The package dns supports (async) querying/replying, incoming/outgoing Axfr/Ixfr, 
// TSIG, EDNS0, dynamic updates, notifies and DNSSEC validation/signing.
// Note that domain names MUST be full qualified, before sending them.
//
// Resource records are native types. They are not stored in wire format.
// Basic usage pattern for creating a new resource record:
//
//      r := new(dns.RR_TXT)
//      r.Hdr = dns.RR_Header{Name: "miek.nl.", Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: 3600}
//      r.Pref = 10
//      r.Mx = "mx.miek.nl."
//
// Or directly from a string:
//
//      mx, err := dns.NewRR("miek.nl. 3600 IN MX 10 mx.miek.nl.")
//
// Or when the default TTL (3600) and class (IN) suit you:
//
//      mx, err := dns.NewRR("miek.nl. MX 10 mx.miek.nl.")
//
// Or even:
//
//      mx, err := dns.NewRR("$ORIGIN nl.\nmiek 1H IN MX 10 mx.miek")
// 
//
// In the DNS messages are exchanged, these messages contain resource
// records (sets).  Use pattern for creating a message:
//
//      m := dns.new(Msg)
//      m.SetQuestion("miek.nl.", dns.TypeMX)
//
// The message m is now a message with the question section set to ask
// the MX records for the miek.nl. zone.
//
// The following is slightly more verbose, but more flexible:
//
//      m1 := new(dns.Msg)
//      m1.MsgHdr.Id = Id()
//      m1.MsgHdr.RecursionDesired = false
//      m1.Question = make([]Question, 1)
//      m1.Question[0] = dns.Question{"miek.nl.", dns.TypeMX, dns.ClassINET}
//
// After creating a message it can be send.
// Basic use pattern for synchronous querying the DNS at a
// server configured on 127.0.0.1 and port 53:
//
//      c := new(Client)
//      in, err := c.Exchange(m1, "127.0.0.1:53")
//
// An asynchronous query is also possible, setting up is more elaborate then
// a synchronous query. 
package dns

import (
	"net"
	"strconv"
	"time"
)

const (
	Year68         = 1 << 31 // For RFC1982 (Serial Arithmetic) calculations in 32 bits.
	DefaultMsgSize = 4096    // Standard default for larger than 512 packets.
	UDPMsgSize     = 512     // Default buffer size for servers receiving UDP packets.
	MaxMsgSize     = 65536   // Largest possible DNS packet.
	DefaultTtl     = 3600    // Default TTL.
)

// Error represents a DNS error
type Error struct {
	Err     string
	Name    string
	Server  net.Addr
	Timeout bool
}

func (e *Error) Error() string {
	if e == nil {
		return "dns: <nil>"
	}
	if e.Name == "" {
		return e.Err
	}
	return e.Name + ": " + e.Err

}

// An RR represents a resource record.
type RR interface {
	// Header returns the header of an resource record. The header contains
	// everything up to the rdata.
	Header() *RR_Header
	// String returns the text representation of the resource record.
	String() string
	// Len returns the length (in octects) of the uncompressed RR in wire format.
	Len() int
}

// Exchange is used in (asynchronous) communication with the resolver.
type Exchange struct {
	Request    *Msg          // the question sent
	Reply      *Msg          // the answer to the question that was sent
	Rtt        time.Duration // round trip time
	RemoteAddr net.Addr      // address of the server
	Nxdomain   bool          // True if nodata because name does not exist
	Secure     bool          // True if result is secure
	Bogus      bool          // True if result is bogus, see Error for the reason
	Error      error         // if something went wrong, this contains the error
}

// DNS resource records.
// There are many types of messages,
// but they all share the same header.
type RR_Header struct {
	Name     string `dns:"cdomain-name"`
	Rrtype   uint16
	Class    uint16
	Ttl      uint32
	Rdlength uint16 // length of data after header
}

func (h *RR_Header) Header() *RR_Header {
	return h
}

func (h *RR_Header) copy() *RR_Header {
	r := new(RR_Header)
	r.Name = h.Name
	r.Rrtype = h.Rrtype
	r.Class = h.Class
	r.Ttl = h.Ttl
	r.Rdlength = h.Rdlength
	return r
}

func (h *RR_Header) String() string {
	var s string

	if h.Rrtype == TypeOPT {
		s = ";"
		// and maybe other things
	}

	if len(h.Name) == 0 {
		s += ".\t"
	} else {
		s += h.Name + "\t"
	}
	s = s + strconv.FormatInt(int64(h.Ttl), 10) + "\t"

	if _, ok := Class_str[h.Class]; ok {
		s += Class_str[h.Class] + "\t"
	} else {
		s += "CLASS" + strconv.Itoa(int(h.Class)) + "\t"
	}

	if _, ok := Rr_str[h.Rrtype]; ok {
		s += Rr_str[h.Rrtype] + "\t"
	} else {
		s += "TYPE" + strconv.Itoa(int(h.Rrtype)) + "\t"
	}
	return s
}

func (h *RR_Header) Len() int {
	l := len(h.Name) + 1
	l += 10 // rrtype(2) + class(2) + ttl(4) + rdlength(2)
	return l
}

func zoneMatch(pattern, zone string) (ok bool) {
	if len(pattern) == 0 {
		return
	}
	if len(zone) == 0 {
		zone = "."
	}
	pattern = Fqdn(pattern)
	zone = Fqdn(zone)
	i := 0
	for {
		ok = pattern[len(pattern)-1-i] == zone[len(zone)-1-i]
		i++

		if !ok {
			break
		}
		if len(pattern)-1-i < 0 || len(zone)-1-i < 0 {
			break
		}

	}
	return
}
