package dict

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/near/borsh-go"

	"github.com/iotaledger/hive.go/core/generics/lo"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/kv"
)

// Dict is an implementation kv.KVStore interface backed by an in-memory map.
// kv.KVStore represents a key-value store
// where both keys and values are arbitrary byte slices.
type Dict map[string][]byte

// MustGet retrieves value by key
func (d Dict) MustGet(key kv.Key) []byte {
	return kv.MustGet(d, key)
}

// MustHas checks if the value exists
func (d Dict) MustHas(key kv.Key) bool {
	return kv.MustHas(d, key)
}

// MustIterate iterated of key/value pairs. In general, non-deterministic
func (d Dict) MustIterate(prefix kv.Key, f func(key kv.Key, value []byte) bool) {
	kv.MustIterate(d, prefix, f)
}

// MustIterateKeys iterated of keys of the dictionary. In general, non-deterministic
func (d Dict) MustIterateKeys(prefix kv.Key, f func(key kv.Key) bool) {
	kv.MustIterateKeys(d, prefix, f)
}

func (d Dict) MustIterateSorted(prefix kv.Key, f func(key kv.Key, value []byte) bool) {
	kv.MustIterateSorted(d, prefix, f)
}

func (d Dict) MustIterateKeysSorted(prefix kv.Key, f func(key kv.Key) bool) {
	kv.MustIterateKeysSorted(d, prefix, f)
}

// New creates new
func New() Dict {
	return make(Dict)
}

// Clone creates clone (deep copy) of Dict
func (d Dict) Clone() Dict {
	clone := make(Dict)
	d.ForEach(func(key kv.Key, value []byte) bool {
		clone.Set(key, lo.CopySlice(value))
		return true
	})
	return clone
}

// FromKVStore convert (copy) any KVStore to dict
func FromKVStore(s kv.KVStoreReader) (Dict, error) {
	d := make(Dict)
	err := s.Iterate(kv.EmptyPrefix, func(k kv.Key, v []byte) bool {
		d[string(k)] = v
		return true
	})
	return d, err
}

func (d Dict) String() string {
	ret := "         Dict:\n"
	for _, key := range d.KeysSorted() {
		val := d[string(key)]
		if len(val) > 80 {
			val = val[:80]
		}
		ret += fmt.Sprintf(
			"           %s: %s ('%s': '%s')\n",
			slice(iotago.EncodeHex([]byte(key))),
			slice(iotago.EncodeHex(val)),
			printable([]byte(key)),
			printable(val),
		)
	}
	return ret
}

func slice(s string) string {
	if len(s) > 44 {
		return s[:10] + "[...]" + s[len(s)-10:]
	}
	return s
}

func printable(s []byte) string {
	for _, c := range s {
		if c < 0x20 || c > 0x7e {
			return "??? binary data ???"
		}
	}
	return string(s)
}

// ForEach iterates non-deterministic!
func (d Dict) ForEach(fun func(key kv.Key, value []byte) bool) {
	for k, v := range d {
		if !fun(kv.Key(k), v) {
			return // abort when callback returns false
		}
	}
}

// IsEmpty returns of it has no records
func (d Dict) IsEmpty() bool {
	return len(d) == 0
}

// Set sets the value for the key
func (d Dict) Set(key kv.Key, value []byte) {
	if value == nil {
		panic("cannot Set(key, nil), use Del() to remove a key/value")
	}
	d[string(key)] = value
}

// Del removes key/value pair
func (d Dict) Del(key kv.Key) {
	delete(d, string(key))
}

// Has checks if key exist
func (d Dict) Has(key kv.Key) (bool, error) {
	_, ok := d[string(key)]
	return ok, nil
}

// Iterate over keys with prefix
func (d Dict) Iterate(prefix kv.Key, f func(key kv.Key, value []byte) bool) error {
	return d.IterateKeys(prefix, func(key kv.Key) bool {
		return f(key, d[string(key)])
	})
}

// IterateKeys over keys with prefix
func (d Dict) IterateKeys(prefix kv.Key, f func(key kv.Key) bool) error {
	for k := range d {
		if !kv.Key(k).HasPrefix(prefix) {
			continue
		}
		if !f(kv.Key(k)) {
			break
		}
	}
	return nil
}

func (d Dict) IterateSorted(prefix kv.Key, f func(key kv.Key, value []byte) bool) error {
	return d.IterateKeysSorted(prefix, func(key kv.Key) bool {
		return f(key, d[string(key)])
	})
}

func (d Dict) IterateKeysSorted(prefix kv.Key, f func(key kv.Key) bool) error {
	for _, k := range d.KeysSorted() {
		if !k.HasPrefix(prefix) {
			continue
		}
		if !f(k) {
			break
		}
	}
	return nil
}

// Get takes a value. Returns nil if key does not exist
func (d Dict) Get(key kv.Key) ([]byte, error) {
	return d[string(key)], nil
}

func (d Dict) Bytes() []byte {
	data, err := borsh.Serialize(d)
	if err != nil {
		panic(err)
	}
	return data
}

func FromBytes(data []byte) (Dict, error) {
	ret := new(Dict)
	err := borsh.Deserialize(ret, data)
	return *ret, err
}

// Keys takes all keys
func (d Dict) Keys() []kv.Key {
	ret := make([]kv.Key, 0)
	for key := range d {
		ret = append(ret, kv.Key(key))
	}
	return ret
}

// KeysSorted takes keys and sorts them
func (d Dict) KeysSorted() []kv.Key {
	k := d.Keys()
	sort.Slice(k, func(i, j int) bool {
		return k[i] < k[j]
	})
	return k
}

// Extend appends another Dict
func (d Dict) Extend(from Dict) {
	for key, value := range from {
		d.Set(kv.Key(key), value)
	}
}

// Hash takes deterministic has of the dict
func (d Dict) Hash() hashing.HashValue {
	keys := d.KeysSorted()
	data := make([][]byte, 0, 2*len(d))
	for _, k := range keys {
		data = append(data, []byte(k))
		v, _ := d.Get(k)
		data = append(data, v)
	}
	return hashing.HashData(data...)
}

func (d Dict) Equals(d1 Dict) bool {
	if len(d) != len(d1) {
		return false
	}
	for k, v := range d {
		v1, ok := d1[k]
		if !ok {
			return false
		}
		if !bytes.Equal(v, v1) {
			return false
		}
	}
	return true
}

// JSONDict is the JSON-compatible representation of a Dict
type JSONDict struct {
	Items []Item
}

// Item is a JSON-compatible representation of a single key-value pair
type Item struct {
	Key   string `json:"key" swagger:"desc(key (hex-encoded))"`
	Value string `json:"value" swagger:"desc(value (hex-encoded))"`
}

// JSONDict returns a JSON-compatible representation of the Dict
func (d Dict) JSONDict() JSONDict {
	j := JSONDict{Items: make([]Item, len(d))}
	for i, k := range d.KeysSorted() {
		j.Items[i].Key = iotago.EncodeHex([]byte(k))
		j.Items[i].Value = iotago.EncodeHex(d[string(k)])
	}
	return j
}

// FromJSONDict returns a dict based off an JSONDict
func FromJSONDict(jsonDict JSONDict) (Dict, error) {
	j := Dict{}

	if jsonDict.Items != nil {
		for _, k := range jsonDict.Items {
			key, err := iotago.DecodeHex(k.Key)
			if err != nil {
				return nil, err
			}

			value, err := iotago.DecodeHex(k.Value)
			if err != nil {
				return nil, err
			}

			j.Set(kv.Key(key), value)
		}
	}

	return j, nil
}

func (d Dict) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.JSONDict())
}

func (d *Dict) UnmarshalJSON(b []byte) error {
	var j JSONDict
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	*d = make(Dict)
	for _, item := range j.Items {
		k, err := iotago.DecodeHex(item.Key)
		if err != nil {
			return err
		}
		v, err := iotago.DecodeHex(item.Value)
		if err != nil {
			return err
		}
		(*d)[string(k)] = v
	}
	return nil
}
