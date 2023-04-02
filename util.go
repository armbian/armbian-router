package redirector

import (
	"math/rand"
	"reflect"
	"strings"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandomSequence is an insecure, but "good enough" random generator.
func RandomSequence(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

var (
	structTags = []string{"json", "yaml", "maxminddb"}
)

// GetValue is a reflection helper like Laravel's data_get
// It lets us use the syntax of some.field.nested in rules.
func GetValue(val any, key string) (any, bool) {
	v := reflect.ValueOf(val)

	keysSplit := strings.Split(key, ".")

	firstKey := keysSplit[0]

	if field := v.FieldByName(firstKey); field.IsValid() {
		if len(keysSplit[1:]) > 0 {
			return GetValue(field.Interface(), strings.Join(keysSplit[1:], "."))
		}

		return field.Interface(), true
	}

	valueType := v.Type()

	for i := 0; i < valueType.NumField(); i++ {
		fieldType := valueType.Field(i)

		for _, tag := range structTags {
			if fieldType.Tag.Get(tag) == firstKey {
				fieldValue := v.Field(i).Interface()

				if len(keysSplit[1:]) > 0 {
					return GetValue(fieldValue, strings.Join(keysSplit[1:], "."))
				}

				return fieldValue, true
			}
		}
	}

	return nil, false
}
