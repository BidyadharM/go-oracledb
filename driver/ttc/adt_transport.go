package ttc

import (
	"database/sql/driver"

	"github.com/oracle/go-oracledb/driver/adt"
	"github.com/oracle/go-oracledb/driver/common"
)

func newTTIOacNamedType(typ *adt.ObjectType, maxLength common.UB4) (*tTIoac, error) {
	if typ == nil || len(typ.TOID) != 16 {
		common.Odl.Error("ADT metadata error")
		return nil, common.NewOracleError(common.ADTMetadataError, nil)
	}
	if typ.TypeVersion < 0 || typ.TypeVersion > int(^uint16(0)) {
		common.Odl.Error("ADT metadata error")
		return nil, common.NewOracleError(common.ADTMetadataError, nil)
	}
	// Named types use a fixed 11-byte OAC maximum, including NULL named-type
	// binds. The collection image length is carried separately in
	// the RXD envelope and must not determine this OAC field.
	oac := newTTIoac(common.DtyNty, 11)
	toid := common.B1Array(append([]byte(nil), typ.TOID...))
	oac.toid = &toid
	oac.versionNumber = common.UB2(typ.TypeVersion)
	return oac, nil
}

func namedTypeForBind(v any) (*adt.ObjectType, bool) {
	switch value := v.(type) {
	case adt.ObjectCollection:
		if value.Object != nil && value.ObjectType != nil {
			return value.ObjectType, true
		}
	case *adt.ObjectCollection:
		if value != nil && value.Object != nil {
			return value.ObjectType, value.ObjectType != nil
		}
	case *adt.Object:
		if value != nil {
			return value.ObjectType, value.ObjectType != nil
		}
	}
	return nil, false
}

func collectionForBind(v any) (adt.ObjectCollection, bool) {
	switch value := v.(type) {
	case adt.ObjectCollection:
		return value, value.Object != nil
	case *adt.ObjectCollection:
		if value != nil && value.Object != nil {
			return *value, true
		}
	}
	return adt.ObjectCollection{}, false
}

// encodeCollectionImage emits the 8.1 collection image used by the Thin
// protocol. Element values reuse the driver's scalar codecs; the descriptor's
// parsed element type guards against binding an unsupported collection shape.
func encodeCollectionImage(collection adt.ObjectCollection, factory codecFactory) (common.B1Array, error) {
	if collection.ObjectType == nil || !collection.ObjectType.Collection {
		common.Odl.Error("ADT collection type error")
		return nil, common.NewOracleError(common.ADTValueError, nil)
	}
	if collection.Null {
		return nil, nil
	}
	if collection.ElementType == 0 {
		common.Odl.Error("ADT metadata error")
		return nil, common.NewOracleError(common.ADTMetadataError, nil)
	}
	// The 8.1 collection image header contains a prefix segment with the
	// inline/type-version flag and the collection's TDS version. For versions
	// up to 245 the version is a two-byte signed value; larger versions use the
	// five-byte pickle-length form.
	version := collection.ObjectType.TypeVersion
	prefix := []byte{0x11}
	if version <= 245 {
		prefix = append(prefix, byte(version>>8), byte(version))
	} else {
		prefix = appendPickleLength(prefix, version)
	}
	data := []byte{0x88, 1, 254, 0, 0, 0, 0}
	data = appendPickleLength(data, len(prefix))
	data = append(data, prefix...)
	data = append(data, 0) // collection flags
	data = appendPickleLength(data, len(collection.Object.Values))
	for _, value := range collection.Object.Values {
		if value == nil {
			data = append(data, 255)
			continue
		}
		var encoder encoderFunc
		var err error
		encoder, err = factory.GetCollectionEncoder(collection.ElementType)
		if err != nil {
			encoder, err = factory.GetEncoder(normalizeBindValue(value))
			if err != nil {
				return nil, err
			}
		}
		encoded, err := encoder(value)
		if err != nil {
			return nil, err
		}
		data = appendPickleLength(data, len(encoded))
		data = append(data, encoded...)
	}
	size := len(data)
	data[3], data[4], data[5], data[6] = byte(size>>24), byte(size>>16), byte(size>>8), byte(size)
	return common.B1Array(data), nil
}

func appendPickleLength(data []byte, length int) []byte {
	if length <= 245 {
		return append(data, byte(length))
	}
	return append(data, 254, byte(length>>24), byte(length>>16), byte(length>>8), byte(length))
}

func decodeCollectionImage(data common.B1Array, typ *adt.ObjectType, factory codecFactory) (driver.Value, error) {
	if len(data) < 10 || data[0]&0x88 != 0x88 {
		common.Odl.Error("ADT collection image error")
		return nil, common.NewOracleError(common.ADTEncodingError, nil)
	}
	// The 8.1 collection header is: image flags/version, image length, prefix
	// segment length, prefix flags and data (including type version), followed
	// by collection flags and the element count.
	_, pos, err := readPickleLength(data, 2)
	if err != nil {
		return nil, err
	}
	prefixLength, next, err := readPickleLength(data, pos)
	if err != nil {
		return nil, err
	}
	pos = next
	if prefixLength < 1 || pos+prefixLength > len(data) {
		common.Odl.Error("ADT collection image error")
		return nil, common.NewOracleError(common.ADTEncodingError, nil)
	}
	pos += prefixLength // prefix flag plus prefix data
	if pos >= len(data) {
		common.Odl.Error("ADT collection image error")
		return nil, common.NewOracleError(common.ADTEncodingError, nil)
	}
	pos++ // collection flags
	count, next, err := readPickleLength(data, pos)
	if err != nil {
		return nil, err
	}
	pos = next
	collection, err := typ.NewCollection()
	if err != nil {
		return nil, err
	}
	decoder, err := factory.GetDecoder(typ.ElementType)
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		if pos >= len(data) {
			common.Odl.Error("ADT collection image error")
			return nil, common.NewOracleError(common.ADTEncodingError, nil)
		}
		if data[pos] == 255 {
			collection.Object.Values = append(collection.Object.Values, nil)
			pos++
			continue
		}
		length, next, err := readPickleLength(data, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		if length < 0 || pos+length > len(data) {
			common.Odl.Error("ADT collection image error")
			return nil, common.NewOracleError(common.ADTEncodingError, nil)
		}
		value, err := decoder.decodeToType(ColumnContext{DataType: typ.ElementType}, data[pos:pos+length])
		if err != nil {
			return nil, err
		}
		collection.Object.Values = append(collection.Object.Values, value)
		pos += length
	}
	return collection, nil
}

func readPickleLength(data []byte, pos int) (int, int, error) {
	if pos >= len(data) {
		common.Odl.Error("ADT collection image error")
		return 0, pos, common.NewOracleError(common.ADTEncodingError, nil)
	}
	if data[pos] <= 245 {
		return int(data[pos]), pos + 1, nil
	}
	if data[pos] != 254 || pos+5 > len(data) {
		common.Odl.Error("ADT collection image error")
		return 0, pos, common.NewOracleError(common.ADTEncodingError, nil)
	}
	return int(uint32(data[pos+1])<<24 | uint32(data[pos+2])<<16 | uint32(data[pos+3])<<8 | uint32(data[pos+4])), pos + 5, nil
}
