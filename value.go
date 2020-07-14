package pack

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing/quick"

	"github.com/renproject/surge"
)

// A Value is a common interface for all values that are able to be marshaled to
// binary and JSON, and are able to express their type information.
type Value interface {
	surge.Marshaler
	json.Marshaler

	Type() Type
}

// A Typed struct is a wrapper around a struct. It includes a type definition
// when marshaled to binary or JSON. This type definition allows for well-typed
// unmarshaling over-the-wire. This is particularly useful when sending
// well-typed values over a network, or when saving them to the disk.
type Typed Struct

// NewTyped returns a well-typed struct from a slice of variadic arguments.
// The arguments are expected to be of the form ("name", value)* otherwise the
// function will panic.
//
//  x := NewTyped(
//      "foo", NewU64(42),
//      "bar", NewString("pack is awesome"),
//      "baz", NewBool(true),
//  )
//
func NewTyped(vs ...interface{}) Typed {
	return Typed(NewStruct(vs...))
}

// Type returns the inner structured record type. This method has O(n)
// complexity, where N is the number of fields in the well-typed struct.
func (typed Typed) Type() Type {
	return Struct(typed).Type()
}

// MarshalJSON marshals the typed value into JSON. It will marshal an object
// with fields "t" and "v". The "t" field defines the type of the "v" field. The
// "v" field is the JSON marshaling of the value.
func (typed Typed) MarshalJSON() ([]byte, error) {
	t, err := json.Marshal(Struct(typed).Type())
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]interface{}{
		"t": map[string]json.RawMessage{"struct": t},
		"v": Struct(typed),
	})
}

// UnmarshalJSON unmarshals the typed value from JSON. It will unmarshal the
// object and expect two fields: "t" and "v". It will use the "t" field to
// understand the type of "v". It will then use this understanding to unmarshal
// "v" into a well-typed struct.
func (typed *Typed) UnmarshalJSON(data []byte) error {
	type Raw struct {
		T json.RawMessage `json:"t"`
		V json.RawMessage `json:"v"`
	}
	raw := Raw{}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("unmarshaling raw: %v", err)
	}
	t, err := unmarshalTypeJSON(raw.T)
	if err != nil {
		return fmt.Errorf("unmarshaling \"t\": %v", err)
	}
	v, err := t.UnmarshalValueJSON(raw.V)
	if err != nil {
		return fmt.Errorf("unmarshaling \"v\": %v", err)
	}
	s, ok := v.(Struct)
	if !ok {
		return fmt.Errorf("expected kind \"struct\", got kind \"%v\"", t.Kind())
	}
	*typed = Typed(s)
	return nil
}

// SizeHint returns the number of bytes required to represent the typed value in
// binary.
func (typed Typed) SizeHint() int {
	return SizeHintType(Struct(typed).Type()) + Struct(typed).SizeHint()
}

// Marshal the typed value into binary. The type definition for the typed values
// will be marshaled first, and then the actual value will be marshaled.
func (typed Typed) Marshal(buf []byte, rem int) ([]byte, int, error) {
	var err error
	if buf, rem, err = MarshalType(Struct(typed).Type(), buf, rem); err != nil {
		return buf, rem, err
	}
	if buf, rem, err = Struct(typed).Marshal(buf, rem); err != nil {
		return buf, rem, err
	}
	return buf, rem, nil
}

// Unmarshal the typed value from binary. The type definition will be
// unmarshaled first, and then this will be used to unmarshal the actual value
// into a well-typed struct.
func (typed *Typed) Unmarshal(buf []byte, rem int) ([]byte, int, error) {
	var err error
	var t Type
	var v Value
	if buf, rem, err = UnmarshalType(&t, buf, rem); err != nil {
		return buf, rem, err
	}
	if v, buf, rem, err = t.UnmarshalValue(buf, rem); err != nil {
		return buf, rem, err
	}
	s, ok := v.(Struct)
	if !ok {
		return buf, rem, fmt.Errorf("expected kind \"struct\", got kind \"%v\"", t.Kind())
	}
	*typed = Typed(s)
	return buf, rem, nil
}

// String returns the struct in its JSON representation.
func (typed Typed) String() string {
	data, err := typed.MarshalJSON()
	if err != nil {
		return fmt.Sprintf(`{"error": %v}`, err)
	}
	return string(data)
}

// Generate a random well-typed struct. This method is implemented for use in
// quick tests. See https://golang.org/pkg/testing/quick/#Generator for more
// information. Generated typed values will never contain embedded structs.
func (Typed) Generate(r *rand.Rand, size int) reflect.Value {
	return reflect.ValueOf(Typed(Struct{}.Generate(r, size).Interface().(Struct)))
}

// Generate a random value. This is helpful when implementing generators for
// other types. See https://golang.org/pkg/testing/quick/#Generator for more
// information.
func Generate(r *rand.Rand, size int, allowStruct bool) reflect.Value {
	t := reflect.Type(nil)
	kind, _ := quick.Value(reflect.TypeOf(Kind(0)), r)
	switch kind.Interface().(Kind) {
	case KindBool:
		t = reflect.TypeOf(Bool(false))
	case KindU8:
		t = reflect.TypeOf(U8{})
	case KindU16:
		t = reflect.TypeOf(U16{})
	case KindU32:
		t = reflect.TypeOf(U32{})
	case KindU64:
		t = reflect.TypeOf(U64{})
	case KindU128:
		t = reflect.TypeOf(U128{})
	case KindU256:
		t = reflect.TypeOf(U256{})
	case KindString:
		t = reflect.TypeOf(String(""))
	case KindBytes:
		t = reflect.TypeOf(Bytes{})
	case KindBytes32:
		t = reflect.TypeOf(Bytes32{})
	case KindBytes65:
		t = reflect.TypeOf(Bytes65{})
	case KindStruct:
		if !allowStruct {
			return Generate(r, size, allowStruct)
		}
		t = reflect.TypeOf(Struct{})
	default:
		panic("non-exhaustive pattern")
	}
	v, _ := quick.Value(t, r)
	return v
}
