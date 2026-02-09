package badger

import (
	"github.com/dgraph-io/badger/v4"
)

// ReadTx executes a read-only transaction.
func ReadTx(db *badger.DB, fn func(txn *badger.Txn) error) error {
	return db.View(fn)
}

// WriteTx executes a read-write transaction.
func WriteTx(db *badger.DB, fn func(txn *badger.Txn) error) error {
	return db.Update(fn)
}

// BatchWrite executes multiple writes in a single batch.
// More efficient than individual writes for bulk operations.
func BatchWrite(db *badger.DB, fn func(wb *badger.WriteBatch) error) error {
	wb := db.NewWriteBatch()
	defer wb.Cancel()

	if err := fn(wb); err != nil {
		return err
	}
	return wb.Flush()
}

// Iterate iterates over keys with a given prefix.
// The callback receives key and value for each matching entry.
func Iterate(db *badger.DB, prefix []byte, fn func(key, value []byte) error) error {
	return db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				return fn(item.Key(), val)
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// IterateKeys iterates over keys only (no value fetching).
// More efficient when values are not needed.
func IterateKeys(db *badger.DB, prefix []byte, fn func(key []byte) error) error {
	return db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = false

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			if err := fn(it.Item().Key()); err != nil {
				return err
			}
		}
		return nil
	})
}

// IterateReverse iterates over keys in reverse order.
func IterateReverse(db *badger.DB, prefix []byte, fn func(key, value []byte) error) error {
	return db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.Reverse = true

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				return fn(item.Key(), val)
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}
