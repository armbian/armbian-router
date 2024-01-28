package util

import (
	"math/rand"
	"reflect"
	"strings"

	"github.com/armbian/redirector/db"
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

func GetValue(val any, key string) (any, bool) {
	// Bypass reflection for known types
	if strings.HasPrefix(key, "asn") || strings.HasPrefix(key, "city") {
		return db.GetValue(val, key)
	}

	// Fallback to reflection
	return getValueReflect(val, key)
}

// GetValue is a reflection helper like Laravel's data_get
// It lets us use the syntax of some.field.nested in rules.
// This is the slow path, see db.GetValue for the faster (somewhat generated) path
func getValueReflect(val any, key string) (any, bool) {
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
