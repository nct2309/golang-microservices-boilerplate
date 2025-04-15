package dto

import (
	"errors"
	"fmt"
	"reflect"
	"time"
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

// applyPartialUpdate applies non-nil fields from a DTO struct (src) to a target entity struct (dst).
// It assumes the convention that pointer fields in the DTO indicate fields to be updated.
// dst must be a pointer to a struct.
func ApplyPartialUpdate(src interface{}, dst interface{}) error {
	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Ptr || dstValue.IsNil() {
		return errors.New("destination must be a non-nil pointer")
	}
	dstElem := dstValue.Elem()
	if dstElem.Kind() != reflect.Struct {
		return errors.New("destination must point to a struct")
	}

	srcValue := reflect.ValueOf(src)
	if srcValue.Kind() == reflect.Ptr {
		if srcValue.IsNil() {
			return nil // Nothing to update from a nil source DTO
		}
		srcValue = srcValue.Elem()
	}
	if srcValue.Kind() != reflect.Struct {
		return errors.New("source must be a struct or pointer to struct")
	}

	srcType := srcValue.Type()
	for i := 0; i < srcValue.NumField(); i++ {
		srcField := srcValue.Field(i)
		srcFieldType := srcType.Field(i)

		// Process only if the source field is a non-nil pointer
		if srcField.Kind() == reflect.Ptr && !srcField.IsNil() {
			// Get the underlying value from the source pointer
			srcElemValue := srcField.Elem()

			// Find the corresponding field in the destination struct by name
			dstField := dstElem.FieldByName(srcFieldType.Name)

			if dstField.IsValid() && dstField.CanSet() {
				// Check if the source element type can be assigned or converted to the destination field type
				if srcElemValue.Type().AssignableTo(dstField.Type()) {
					dstField.Set(srcElemValue)
				} else if srcElemValue.Type().ConvertibleTo(dstField.Type()) {
					dstField.Set(srcElemValue.Convert(dstField.Type()))
				} else {
					// Log or return error for incompatible types if needed
					// This case might indicate a mismatch between DTO and Entity struct definitions
					return fmt.Errorf("cannot assign/convert DTO field '%s' (%s) to entity field (%s)",
						srcFieldType.Name, srcElemValue.Type(), dstField.Type())
				}
			}
		} // Ignore fields in DTO that are nil pointers or not pointers
	}
	return nil
}
