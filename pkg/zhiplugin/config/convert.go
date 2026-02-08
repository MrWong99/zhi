package config

import (
	"fmt"
	"reflect"
	"strconv"
)

// TryConvert attempts to convert the value to the given type.
func (v *Value) TryConvert(targetType reflect.Type) error {
	if reflect.TypeOf(v.Val) == targetType {
		return nil
	}
	switch targetType.Kind() {
	case reflect.String:
		v.Val = fmt.Sprintf("%v", v.Val)
	case reflect.Int:
		switch typedVal := v.Val.(type) {
		case int:
			v.Val = typedVal
		case int8:
			v.Val = int(typedVal)
		case int16:
			v.Val = int(typedVal)
		case int32:
			v.Val = int(typedVal)
		case int64:
			v.Val = int(typedVal)
		case uint:
			v.Val = int(typedVal)
		case uint8:
			v.Val = int(typedVal)
		case uint16:
			v.Val = int(typedVal)
		case uint32:
			v.Val = int(typedVal)
		case uint64:
			v.Val = int(typedVal)
		case float32:
			v.Val = int(typedVal)
		case float64:
			v.Val = int(typedVal)
		case string:
			intVal, err := strconv.Atoi(typedVal)
			if err != nil {
				return err
			}
			v.Val = intVal
		default:
			return fmt.Errorf("cannot convert %T to int", v.Val)
		}
	case reflect.Uint:
		switch typedVal := v.Val.(type) {
		case int:
			v.Val = uint(typedVal)
		case int8:
			v.Val = uint(typedVal)
		case int16:
			v.Val = uint(typedVal)
		case int32:
			v.Val = uint(typedVal)
		case int64:
			v.Val = uint(typedVal)
		case uint:
			v.Val = typedVal
		case uint8:
			v.Val = uint(typedVal)
		case uint16:
			v.Val = uint(typedVal)
		case uint32:
			v.Val = uint(typedVal)
		case uint64:
			v.Val = uint(typedVal)
		case float32:
			v.Val = uint(typedVal)
		case float64:
			v.Val = uint(typedVal)
		case string:
			intVal, err := strconv.Atoi(typedVal)
			if err != nil {
				return err
			}
			v.Val = uint(intVal)
		default:
			return fmt.Errorf("cannot convert %T to uint", v.Val)
		}
	case reflect.Float64:
		switch typedVal := v.Val.(type) {
		case int:
			v.Val = float64(typedVal)
		case int8:
			v.Val = float64(typedVal)
		case int16:
			v.Val = float64(typedVal)
		case int32:
			v.Val = float64(typedVal)
		case int64:
			v.Val = float64(typedVal)
		case uint:
			v.Val = float64(typedVal)
		case uint8:
			v.Val = float64(typedVal)
		case uint16:
			v.Val = float64(typedVal)
		case uint32:
			v.Val = float64(typedVal)
		case uint64:
			v.Val = float64(typedVal)
		case float32:
			v.Val = float64(typedVal)
		case float64:
			v.Val = float64(typedVal)
		case string:
			floatVal, err := strconv.ParseFloat(typedVal, 64)
			if err != nil {
				return err
			}
			v.Val = float64(floatVal)
		default:
			return fmt.Errorf("cannot convert %T to float", v.Val)
		}
	case reflect.Bool:
		switch typedVal := v.Val.(type) {
		case int:
			v.Val = typedVal != 0
		case int8:
			v.Val = typedVal != 0
		case int16:
			v.Val = typedVal != 0
		case int32:
			v.Val = typedVal != 0
		case int64:
			v.Val = typedVal != 0
		case uint:
			v.Val = typedVal != 0
		case uint8:
			v.Val = typedVal != 0
		case uint16:
			v.Val = typedVal != 0
		case uint32:
			v.Val = typedVal != 0
		case uint64:
			v.Val = typedVal != 0
		case float32:
			v.Val = typedVal != 0
		case float64:
			v.Val = typedVal != 0
		case string:
			boolVal, err := strconv.ParseBool(typedVal)
			if err != nil {
				return err
			}
			v.Val = boolVal
		default:
			return fmt.Errorf("cannot convert %T to bool", v.Val)
		}
	default:
		return fmt.Errorf("cannot convert %T to %s", v.Val, targetType)
	}

	return nil
}
