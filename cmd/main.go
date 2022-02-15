package cmd

import (
	"bytes"
	"encoding/gob"
	"flag"
	"fmt"
	"os"

	"github.com/Mrs4s/go-cqhttp/db"
	"github.com/Mrs4s/go-cqhttp/global"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
)

func handle(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func Main() {
	v2 := flag.String("from", "data/leveldb-v2", "leveldb v2 path")
	v3 := flag.String("to", "data/leveldb-v3", "leveldb v3 path")
	flag.Parse()
	if flag.Arg(0) == "help" {
		flag.Usage()
		return
	}
	gob.Register(db.StoredMessageAttribute{}) // register gob types
	gob.Register(db.StoredGuildMessageAttribute{})
	gob.Register(db.QuotedInfo{})
	gob.Register(global.MSG{})
	gob.Register(db.StoredGroupMessage{})
	gob.Register(db.StoredPrivateMessage{})
	gob.Register(db.StoredGuildChannelMessage{})

	v2db, err := leveldb.OpenFile(*v2, nil)
	handle(err)
	defer v2db.Close()
	{
		v3db, err := leveldb.OpenFile(*v3, nil)
		handle(err)
		_ = v3db.Close()
	}
	v3db, err := leveldb.OpenFile(*v3, nil)
	handle(err)

	iter := v2db.NewIterator(nil, nil)
	var errs []error
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		if len(value) == 0 {
			continue
		}
		writer := newWriter()
		data := value[1:]
		writer.uvarint(uint64(value[0]))
		switch value[0] {
		case group:
			x := &db.StoredGroupMessage{}
			if err = gob.NewDecoder(bytes.NewReader(data)).Decode(x); err != nil {
				errs = append(errs, errors.Wrap(err, "decode group message failed"))
				continue
			}
			writer.writeStoredGroupMessage(x)
		case private:
			x := &db.StoredPrivateMessage{}
			if err = gob.NewDecoder(bytes.NewReader(data)).Decode(x); err != nil {
				errs = append(errs, errors.Wrap(err, "decode private message failed"))
				continue
			}
			writer.writeStoredPrivateMessage(x)
		case guildChannel:
			x := &db.StoredGuildChannelMessage{}
			if err = gob.NewDecoder(bytes.NewReader(data)).Decode(x); err != nil {
				errs = append(errs, errors.Wrap(err, "decode guild channel message failed"))
				continue
			}
			writer.writeStoredGuildChannelMessage(x)
		}
		err := v3db.Put(key, writer.bytes(), nil)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "put to v3 failed"))
		}
	}
	iter.Release()
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Printf("%v\n", err)
		}
	}
	tr, err := v3db.OpenTransaction()
	handle(err)
	handle(tr.Commit())
	handle(v3db.Close())
}
