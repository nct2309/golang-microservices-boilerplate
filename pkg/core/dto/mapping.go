package dto

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// MapToEntity copies data from a source DTO (or any struct/pointer) to a destination entity pointer.
// It performs a shallow copy based on matching field names, handling pointer conversions.
// Destination `to` must be a pointer to a struct.
func MapToEntity(from interface{}, to interface{}) error {
	fromValue := reflect.ValueOf(from)
	toValuePtr := reflect.ValueOf(to)

	if toValuePtr.Kind() != reflect.Ptr || toValuePtr.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer")
	}
	toValue := toValuePtr.Elem()
	if toValue.Kind() != reflect.Struct {
		return fmt.Errorf("destination must point to a struct")
	}

	// Handle if 'from' is a pointer, dereference it
	if fromValue.Kind() == reflect.Ptr {
		if fromValue.IsNil() {
			// If source is nil pointer, nothing to copy
			return nil
		}
		fromValue = fromValue.Elem()
	}

	if fromValue.Kind() != reflect.Struct {
		return fmt.Errorf("source must be a struct or a pointer to a struct")
	}

	fromType := fromValue.Type()
	for i := 0; i < fromValue.NumField(); i++ {
		fromFieldName := fromType.Field(i).Name
		fromFieldValue := fromValue.Field(i)

		// Skip unexported fields
		if !fromFieldValue.CanInterface() {
			continue
		}

		toFieldValue := toValue.FieldByName(fromFieldName)

		if toFieldValue.IsValid() && toFieldValue.CanSet() {
			isFromPtr := fromFieldValue.Kind() == reflect.Ptr
			isToPtr := toFieldValue.Kind() == reflect.Ptr

			// Case 1: Direct assignment possible (types match exactly)
			if fromFieldValue.Type().AssignableTo(toFieldValue.Type()) {
				toFieldValue.Set(fromFieldValue)
				continue
			}

			// Case 2: Destination is Ptr, Source is not Ptr
			if isToPtr && !isFromPtr {
				// Check if Source type is assignable to Dest Elem type
				if fromFieldValue.Type().AssignableTo(toFieldValue.Type().Elem()) {
					newPtr := reflect.New(toFieldValue.Type().Elem()) // Create new pointer of destination element type
					newPtr.Elem().Set(fromFieldValue)                 // Set pointer's element value
					toFieldValue.Set(newPtr)                          // Assign the new pointer
					continue
				}
			}

			// Case 3: Source is Ptr, Destination is not Ptr
			if isFromPtr && !isToPtr {
				if !fromFieldValue.IsNil() {
					fromElemValue := fromFieldValue.Elem() // Dereference source pointer
					// Check if Source Elem type is assignable/convertible to Dest type
					if fromElemValue.Type().AssignableTo(toFieldValue.Type()) {
						toFieldValue.Set(fromElemValue)
						continue
					} else if fromElemValue.Type().ConvertibleTo(toFieldValue.Type()) {
						toFieldValue.Set(fromElemValue.Convert(toFieldValue.Type()))
						continue
					}
				}
				// If fromFieldValue is Nil, do nothing for non-ptr destination
				continue
			}

			// Case 4: Basic type conversion (neither is pointer, types differ)
			if !isFromPtr && !isToPtr {
				if fromFieldValue.Type().ConvertibleTo(toFieldValue.Type()) {
					toFieldValue.Set(fromFieldValue.Convert(toFieldValue.Type()))
					continue
				}
			}

			// Case 5: Nested Structs (Recursive Call)
			if fromFieldValue.Kind() == reflect.Struct && toFieldValue.Kind() == reflect.Struct {
				// Ensure destination field is addressable for the recursive call
				if toFieldValue.CanAddr() {
					if err := MapToEntity(fromFieldValue.Interface(), toFieldValue.Addr().Interface()); err != nil {
						// Potentially log or wrap this error, returning prevents further mapping
						return fmt.Errorf("error mapping nested struct field '%s': %w", fromFieldName, err)
					}
					continue
				}
			} else if fromFieldValue.Kind() == reflect.Ptr && fromFieldValue.Elem().Kind() == reflect.Struct &&
				toFieldValue.Kind() == reflect.Ptr && toFieldValue.Type().Elem().Kind() == reflect.Struct {
				// Handle pointer to struct -> pointer to struct
				if !fromFieldValue.IsNil() {
					if toFieldValue.IsNil() {
						// If destination is nil, create a new struct instance for it
						newStructPtr := reflect.New(toFieldValue.Type().Elem())
						toFieldValue.Set(newStructPtr)
					}
					if err := MapToEntity(fromFieldValue.Interface(), toFieldValue.Interface()); err != nil {
						return fmt.Errorf("error mapping nested pointer to struct field '%s': %w", fromFieldName, err)
					}
				} else {
					// If source is nil, set destination to nil
					toFieldValue.Set(reflect.Zero(toFieldValue.Type()))
				}
				continue
			}

			// Case 6: Explicit time.Time check (often handled by AssignableTo, but good for clarity)
			if _, ok := fromFieldValue.Interface().(time.Time); ok {
				if fromFieldValue.Type().AssignableTo(toFieldValue.Type()) {
					toFieldValue.Set(fromFieldValue)
					continue
				}
			}

			// Add more specific type handling if needed (e.g., custom type conversions not covered by ConvertibleTo)
			// Example: if fromFieldValue.Type() == reflect.TypeOf(MyCustomType{}) { ... }
		}
	}

	return nil
}

// MapToDTO copies data from a source entity (or any struct/pointer) to a destination DTO pointer.
// It performs a shallow copy based on matching field names and compatible types.
// Destination `to` must be a pointer to a struct.
func MapToDTO(from interface{}, to interface{}) error {
	// This function is often very similar to MapToEntity.
	// We can reuse the same logic by calling MapToEntity.
	// If different logic is ever needed (e.g., special handling for DTO fields),
	// this function can be implemented separately.
	return MapToEntity(from, to)
}

// ConvertInterfaceToAny converts an interface to a protobuf Any type.
func ConvertInterfaceToAny(v interface{}) (*any.Any, error) {
	anyValue := &any.Any{}
	bytes, _ := json.Marshal(v)
	bytesValue := &wrappers.BytesValue{
		Value: bytes,
	}
	err := anypb.MarshalFrom(anyValue, bytesValue, proto.MarshalOptions{})
	return anyValue, err
}

// ConvertAnyToInterface converts a protobuf Any type to an interface.
func ConvertAnyToInterface(anyValue *any.Any) (interface{}, error) {
	var value interface{}
	bytesValue := &wrappers.BytesValue{}
	err := anypb.UnmarshalTo(anyValue, bytesValue, proto.UnmarshalOptions{})
	if err != nil {
		return value, err
	}
	uErr := json.Unmarshal(bytesValue.Value, &value)
	if uErr != nil {
		return value, uErr
	}
	return value, nil
}
