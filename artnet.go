package artnet

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/projecthunt/reuseable"
)

var lastseq uint8

const notan string = "Not an Art-Net packet"

type OpCode uint16

const (
	OPCODEPOOL      OpCode = 0x20
	OPCODEPOOLREPLY OpCode = 0x21
	OPCODECHANNELS  OpCode = 0x50
)

func notanerr(s string) error {
	return errors.New(fmt.Sprintf("%s (%s).\n", notan, s))
}

type ArtNet struct {
	Type        string
	OpCode      OpCode
	ProtocolVer uint16
	Sequence    uint8
	Physical    uint8
	Port        uint16
	Length      uint16
	Data        [512]uint8
	Pool        ArtNetPoll
	PoolReply   ArtNetPollReply
	Source      net.Addr
	HasChannels bool
}

func (an ArtNet) ChannelsAsString(from, amount int, delim string) string {
	if !an.HasChannels {
		return ""
	}
	ch := make([]string, amount)
	for i, v := range an.Data[from-1 : from-1+amount] {
		ch[i] = strconv.Itoa(int(v))
	}
	return strings.Join(ch, delim)
}

func (an ArtNet) ChannelsAsSliceOfStrings(from, amount int) []string {
	if !an.HasChannels {
		return nil
	}
	ch := make([]string, amount)
	for i, v := range an.Data[from-1 : from-1+amount] {
		ch[i] = strconv.Itoa(int(v))
	}
	return ch
}

func (an ArtNet) ChannelsAsSlice(from, amount int) []uint8 {
	if !an.HasChannels {
		return nil
	}
	ch := make([]uint8, amount)
	for i, v := range an.Data[from-1 : from-1+amount] {
		ch[i] = v
	}
	return ch
}

func (an ArtNet) Channel(n uint) (uint8, bool) {
	if !an.HasChannels {
		return 0, false
	}
	return an.Data[n-1], true
}

type ArtNetPoll struct {
	Priority       uint8
	TxDiagOnChange bool
	TxDiag         bool
	TxDiagUnicast  bool
	TxVLC          bool
}

func (anp ArtNetPoll) ExplainFlags() []string {
	s := make([]string, 0)
	v := "only in response to ArtPoll"
	if anp.TxDiagOnChange {
		v = "on status change"
	}
	s = append(s, fmt.Sprintf("Transmit ArtPollReply %s.", v))
	v = "Do not t"
	if anp.TxDiag {
		v = "T"
	}
	s = append(s, fmt.Sprintf("%sransmit diagnostic messages.", v))
	v = "broadcast"
	if anp.TxDiagUnicast {
		v = "unicast to sender of ArtPoll packet."
	}
	s = append(s, fmt.Sprintf("Diagnostic messages are %s.\n", v))
	v = "ignore ArtVlc packets."
	if anp.TxVLC {
		v = "transmit VLC data."
	}
	s = append(s, fmt.Sprintf("Node should %s.", v))
	return s
}

type ArtNetPollReply struct {
	IPAddress []uint8
	Version   []uint8
	OEM       []uint8
	Name      string
	LongName  string
	Status    string
}

func (apr ArtNetPollReply) IP() string {
	ip := apr.IPAddress
	return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
}

func GetAndParse(debug bool) (ArtNet, error) {
	pc, err := reuseable.ListenPacket("udp", ":6454")
	if err != nil {
		return ArtNet{}, err
	}
	defer pc.Close()
	buf := make([]byte, 1024)
	n, addr, err := pc.ReadFrom(buf)
	return Parse(buf, n, addr, debug)
}

func parsePool(s ArtNet, an []uint8, plog bool) (ArtNet, error) {
	flags := an[4]
	pr := an[5]
	s.Pool = ArtNetPoll{}
	s.Pool.Priority = pr
	if plog {
		log.Println("ArtPoll packet.")
	}
	if flags > 0 {
		txstatchange := flags&(1<<1) > 0
		txdiag := flags&(1<<2) > 0
		diaguni := flags&(1<<3) > 0
		txvlc := flags&(1<<4) > 0
		s.Pool.TxDiagOnChange = txstatchange
		s.Pool.TxDiag = txdiag
		s.Pool.TxDiagUnicast = diaguni
		s.Pool.TxVLC = txvlc
	}
	return s, nil
}

func parseReply(s ArtNet, an, buf []uint8, plog bool) (ArtNet, error) {
	s.PoolReply = ArtNetPollReply{}
	ip := make([]uint8, 4)
	ip[0] = an[2]
	ip[1] = an[3]
	ip[2] = an[4]
	ip[3] = an[5]
	s.PoolReply.IPAddress = ip
	s.PoolReply.Name = strings.Trim(string(buf[26:44]), "\x00")
	s.PoolReply.LongName = strings.Trim(string(buf[44:108]), "\x00")
	s.PoolReply.Status = strings.Trim(string(buf[108:172]), "\x00")
	if plog {
		log.Printf("ArtPollReply: %q @ %s (%q).\n", s.PoolReply.Name, s.PoolReply.IP(), s.PoolReply.Status)
	}
	return s, nil
}

func parseChannels(s ArtNet, an []uint8, plog bool) (ArtNet, error) {
	s.Sequence = an[4]
	lastseq = s.Sequence
	s.Physical = an[5]
	subuni := an[6]
	net := an[7]
	s.Port = uint16(net)*256 + uint16(subuni)
	s.Length = uint16(an[8])*256 + uint16(an[9])
	if s.Length > 512 {
		return s, errors.New(fmt.Sprintf("Invalid packet (data length is %d).\n", s.Length))
	}
	for i, v := range an[10 : 10+s.Length] {
		s.Data[i] = v
	}
	if plog {
		log.Printf("#%3d PHY%d P%d. 1-16/%d: %s\n", s.Sequence, s.Physical, s.Port, s.Length, s.ChannelsAsString(0, 16, " "))
	}
	s.HasChannels = true
	return s, nil
}

func Parse(buf []byte, n int, addr net.Addr, plog bool) (ArtNet, error) {
	s := ArtNet{}
	s.Source = addr
	if n < 14 {
		return s, notanerr(fmt.Sprintf("too short, %d bytes", n))
	}
	isan := string(buf[:8]) == "Art-Net\x00"
	if !isan {
		return s, notanerr(fmt.Sprintf("header is %q", buf[:8]))
	}
	an := make([]uint8, n-8)
	for i, b := range buf[8:n] {
		an[i] = uint8(b)
	}
	op := OpCode(uint16(an[0])*256 + uint16(an[1]))
	ver := uint16(an[2])*256 + uint16(an[3])
	s.OpCode = OpCode(op)
	s.ProtocolVer = ver
	if ver < 14 {
		return s, notanerr(fmt.Sprintf("version is %d", ver))
	}
	switch op {
	case OPCODEPOOL:
		return parsePool(s, an, plog)
	case OPCODEPOOLREPLY:
		return parseReply(s, an, buf, plog)
	case OPCODECHANNELS:
		return parseChannels(s, an, plog)
	}
	return s, errors.New(fmt.Sprintf("Unsupported opcode %d.", op))
}
