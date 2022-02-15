package cmd

import (
	"bytes"
	"io"

	"github.com/Mrs4s/go-cqhttp/db"
	"github.com/Mrs4s/go-cqhttp/global"
)

const dataVersion = 1

const (
	group        = 0x0
	private      = 0x1
	guildChannel = 0x2
)

type coder byte

const (
	coderNil coder = iota
	coderInt
	coderUint
	coderInt32
	coderUint32
	coderInt64
	coderUint64
	coderString
	coderMSG      // global.MSG
	coderArrayMSG // []global.MSG
	coderStruct   // struct{}
)

type intWriter struct {
	bytes.Buffer
}

func (w *intWriter) varint(x int64) {
	w.uvarint(uint64(x)<<1 ^ uint64(x>>63))
}

func (w *intWriter) uvarint(x uint64) {
	for x >= 0x80 {
		w.WriteByte(byte(x) | 0x80)
		x >>= 7
	}
	w.WriteByte(byte(x))
}

// writer implements the index write.
// data format(use uvarint to encode integers):
// | version | string data length | index data length | string data | index data |
// for string data part, each string is encoded as:
// | string length | string |
// for index data part, each value is encoded as:
// | coder | value |
// * coder is the identifier of value's type.
// * specially for string, it's value is the offset in string data part.
type writer struct {
	data        intWriter
	strings     intWriter
	stringIndex map[string]uint64
}

func newWriter() *writer {
	return &writer{
		stringIndex: make(map[string]uint64),
	}
}

func (w *writer) coder(o coder)    { w.data.WriteByte(byte(o)) }
func (w *writer) varint(x int64)   { w.data.varint(x) }
func (w *writer) uvarint(x uint64) { w.data.uvarint(x) }
func (w *writer) nil()             { w.coder(coderNil) }

func (w *writer) int(i int) {
	w.varint(int64(i))
}

func (w *writer) uint(i uint) {
	w.uvarint(uint64(i))
}

func (w *writer) int32(i int32) {
	w.varint(int64(i))
}

func (w *writer) uint32(i uint32) {
	w.uvarint(uint64(i))
}

func (w *writer) int64(i int64) {
	w.varint(i)
}

func (w *writer) uint64(i uint64) {
	w.uvarint(i)
}

func (w *writer) string(s string) {
	off, ok := w.stringIndex[s]
	if !ok {
		// not found write to string data part
		// | string length | string |
		off = uint64(w.strings.Len())
		w.strings.uvarint(uint64(len(s)))
		_, _ = w.strings.WriteString(s)
		w.stringIndex[s] = off
	}
	// write offset to index data part
	w.uvarint(off)
}

func (w *writer) msg(m global.MSG) {
	w.uvarint(uint64(len(m)))
	for s, obj := range m {
		w.string(s)
		w.obj(obj)
	}
}

func (w *writer) arrayMsg(a []global.MSG) {
	w.uvarint(uint64(len(a)))
	for _, v := range a {
		w.msg(v)
	}
}

func (w *writer) obj(o interface{}) {
	switch x := o.(type) {
	case nil:
		w.nil()
	case int:
		w.coder(coderInt)
		w.int(x)
	case int32:
		w.coder(coderInt32)
		w.int32(x)
	case int64:
		w.coder(coderInt64)
		w.int64(x)
	case uint:
		w.coder(coderUint)
		w.uint(x)
	case uint32:
		w.coder(coderUint32)
		w.uint32(x)
	case uint64:
		w.coder(coderUint64)
		w.uint64(x)
	case string:
		w.coder(coderString)
		w.string(x)
	case global.MSG:
		w.coder(coderMSG)
		w.msg(x)
	case []global.MSG:
		w.coder(coderArrayMSG)
		w.arrayMsg(x)
	default:
		panic("unsupported type")
	}
}

func (w *writer) bytes() []byte {
	var out intWriter
	out.uvarint(dataVersion)
	out.uvarint(uint64(w.strings.Len()))
	out.uvarint(uint64(w.data.Len()))
	_, _ = io.Copy(&out, &w.strings)
	_, _ = io.Copy(&out, &w.data)
	return out.Bytes()
}

func (w *writer) writeStoredGroupMessage(x *db.StoredGroupMessage) {
	if x == nil {
		w.nil()
		return
	}
	w.coder(coderStruct)
	w.string(x.ID)
	w.int32(x.GlobalID)
	w.writeStoredMessageAttribute(x.Attribute)
	w.string(x.SubType)
	w.writeQuotedInfo(x.QuotedInfo)
	w.int64(x.GroupCode)
	w.string(x.AnonymousID)
	w.arrayMsg(x.Content)
}

func (w *writer) writeStoredPrivateMessage(x *db.StoredPrivateMessage) {
	if x == nil {
		w.nil()
		return
	}
	w.coder(coderStruct)
	w.string(x.ID)
	w.int32(x.GlobalID)
	w.writeStoredMessageAttribute(x.Attribute)
	w.string(x.SubType)
	w.writeQuotedInfo(x.QuotedInfo)
	w.int64(x.SessionUin)
	w.int64(x.TargetUin)
	w.arrayMsg(x.Content)
}

func (w *writer) writeStoredGuildChannelMessage(x *db.StoredGuildChannelMessage) {
	if x == nil {
		w.nil()
		return
	}
	w.coder(coderStruct)
	w.string(x.ID)
	w.writeStoredGuildMessageAttribute(x.Attribute)
	w.uint64(x.GuildID)
	w.uint64(x.ChannelID)
	w.writeQuotedInfo(x.QuotedInfo)
	w.arrayMsg(x.Content)
}

func (w *writer) writeStoredMessageAttribute(x *db.StoredMessageAttribute) {
	if x == nil {
		w.nil()
		return
	}
	w.coder(coderStruct)
	w.int32(x.MessageSeq)
	w.int32(x.InternalID)
	w.int64(x.SenderUin)
	w.string(x.SenderName)
	w.int64(x.Timestamp)
}

func (w *writer) writeStoredGuildMessageAttribute(x *db.StoredGuildMessageAttribute) {
	if x == nil {
		w.nil()
		return
	}
	w.coder(coderStruct)
	w.uint64(x.MessageSeq)
	w.uint64(x.InternalID)
	w.uint64(x.SenderTinyID)
	w.string(x.SenderName)
	w.int64(x.Timestamp)
}

func (w *writer) writeQuotedInfo(x *db.QuotedInfo) {
	if x == nil {
		w.nil()
		return
	}
	w.coder(coderStruct)
	w.string(x.PrevID)
	w.int32(x.PrevGlobalID)
	w.arrayMsg(x.QuotedContent)
}
