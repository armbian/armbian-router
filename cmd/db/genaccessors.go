package main

import (
	"fmt"
	"github.com/armbian/redirector/geo"
	"github.com/samber/lo"
	"reflect"
	"strings"
)

var (
	structTags = []string{"json", "yaml", "maxminddb"}
)

// This is a VERY messy way to generate static recursive field getters, which transform rule strings
// into the field value
func main() {
	var asn geo.ASN

	accessors(asn)

	var city geo.City

	accessors(city)
	accessors(city.Continent)
	accessors(city.Country)
	accessors(city.Location)
	accessors(city.RegisteredCountry)
}

func accessors(val any) {
	v := reflect.ValueOf(val)

	funcName := fmt.Sprintf("get%s", v.Type().Name())

	valueType := v.Type()

	// Check if we need to include an index check

	var s strings.Builder

	s.WriteString(fmt.Sprintf("func %s(v %s, keys []string) (any, bool) {\n", funcName, valueType.Name()))
	s.WriteString("\tkey := keys[0]\n\n")

	s.WriteString("\tswitch key {\n")

	for i := 0; i < valueType.NumField(); i++ {
		fieldType := valueType.Field(i)

		var fieldNames = []string{fieldType.Name}

		for _, tag := range structTags {
			// Append tags to possible names
			tagVal := fieldType.Tag.Get(tag)

			if tagVal == "" {
				continue
			}

			fieldNames = append(fieldNames, tagVal)
		}

		s.WriteString("\t\tcase ")

		formattedNames := lo.Map(lo.Uniq(fieldNames), func(name string, _ int) string {
			return fmt.Sprintf(`"%s"`, name)
		})

		s.WriteString(strings.Join(formattedNames, ", "))
		s.WriteString(":\n")

		// If struct/map/etc, access. If string/other value, return.
		s.WriteString("\t\t\t")

		// Check kind as it's an int, starting at bool and ending at Complex128. Then include String.
		if fieldType.Type.Kind() >= reflect.Bool && fieldType.Type.Kind() <= reflect.Complex128 ||
			fieldType.Type.Kind() == reflect.String {
			s.WriteString("return ")
			s.WriteString("v.")
			s.WriteString(fieldType.Name)
			s.WriteString(", true")
		} else if fieldType.Type.Kind() == reflect.Map {
			// Handle slice logic (index, so [0] off the key)
			s.WriteString("index := getMapIndex(key)\n\n")

			s.WriteString("\t\t\tif index == \"\" {\n")
			s.WriteString("\t\t\t\treturn nil, false\n")
			s.WriteString("\t\t\t}\n\n")

			s.WriteString("\t\t\tm, found := v." + fieldType.Name + "[index]\n")

			s.WriteString("\t\t\treturn m, found")
		} else if fieldType.Type.Kind() == reflect.Slice {
			// Handle slice logic (index, so [0] off the key)
			s.WriteString("index := getSliceIndex(key)\n\n")

			s.WriteString("\t\t\tif index == nil {\n")
			s.WriteString("\t\t\t\treturn nil, false\n")
			s.WriteString("\t\t\t}\n\n")

			s.WriteString(fmt.Sprintf("\t\t\treturn v.%s[index]", fieldType.Name))
		} else if fieldType.Type.Kind() == reflect.Struct {
			s.WriteString("return get" + fieldType.Type.Name() + "(v." + fieldType.Name + ", keys[1:])")
		} else {
			s.WriteString("return nil, true")
		}
		s.WriteString("\n")
	}

	s.WriteString("\t}\n\n")
	s.WriteString("\treturn nil, false\n")
	s.WriteString("}\n")

	fmt.Println(s.String())
}
