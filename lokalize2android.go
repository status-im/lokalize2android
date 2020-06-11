package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"text/scanner"
)

type Resources struct {
	Strings      []String      `xml:"string"`
	StringArrays []StringArray `xml:"string-array"`
	Plurals      []Plurals     `xml:"plural"`

	XMLName struct{} `xml:"resources"`
}

type String struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",innerxml"`
}

type StringArray struct {
	Name  string   `xml:"name,attr"`
	Items []string `xml:"item"`
}

type Plurals struct {
	Name  string       `xml:"name,attr"`
	Items []PluralItem `xml:"item"`
}

type PluralItem struct {
	Quantity string `xml:"quantity,attr"`
	Value    string `xml:",innerxml"`
}

var pluralTypes = []string{"zero", "one", "two", "few", "many", "other"}

func (l *Resources) UnmarshalJSON(b []byte) error {
	var ts map[string]interface{}
	if err := json.Unmarshal(b, &ts); err != nil {
		return err
	}

	for k, v := range ts {
		switch v.(type) {
		case string:
			str := String{Name: processKey(k), Value: processTranslation(v.(string))}
			l.Strings = append(l.Strings, str)
		case []interface{}:
			strs := StringArray{Name: processKey(k)}
			for _, str := range v.([]interface{}) {
				strs.Items = append(strs.Items, processTranslation(str.(string)))
			}
			l.StringArrays = append(l.StringArrays, strs)
		case map[string]interface{}:
			pl := Plurals{Name: processKey(k)}
			for _, pt := range pluralTypes {
				if str, ok := v.(map[string]interface{})[pt]; ok {
					pl.Items = append(pl.Items, PluralItem{Quantity: pt, Value: processTranslation(str.(string))})
				}
			}
			l.Plurals = append(l.Plurals, pl)
		default:
			return fmt.Errorf("can't handle %q: %q", k, v)
		}
	}

	return nil
}

func processKey(k string) string {
	return strings.ReplaceAll(k, "-", "_")
}

// processTranslation transforms {{ and }} into XML surrounding tags for placeholders.
func processTranslation(v string) string {
	var (
		s  scanner.Scanner
		b  bytes.Buffer
		ph bytes.Buffer
	)

	// TODO(andremedeiros): can this be done better?
	v = strings.ReplaceAll(v, "{{", "{")
	v = strings.ReplaceAll(v, "}}", "}")

	s.Init(strings.NewReader(v))
	s.Filename = "translation"
	s.Whitespace = 0
	s.Mode = scanner.ScanStrings
	s.IsIdentRune = func(ch rune, i int) bool {
		return ch == '{' || ch == '}'
	}

	insideph := false
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch tok {
		case '{':
			insideph = true
		case '}':
			b.WriteString(fmt.Sprintf(`<xliff:g id="%s" />`, ph.String()))
			ph.Reset()
			insideph = false
		default:
			if !insideph {
				b.WriteRune(tok)
			} else {
				ph.WriteRune(tok)
			}
		}
	}
	return b.String()
}

func main() {
	l := log.New(os.Stderr, "", log.Flags())

	var r io.Reader
	if len(os.Args) == 2 {
		f, err := os.Open(os.Args[1])
		if err != nil {
			l.Fatalf("error reading file: %q", err)
		}

		defer func() {
			err = f.Close()
			if err != nil {
				l.Fatalf("error closing file: %q", err)
			}
		}()

		r = f
	} else {
		r = bufio.NewReader(os.Stdin)
	}

	var rs Resources
	if err := json.NewDecoder(r).Decode(&rs); err != nil {
		l.Fatalf("error parsing json: %q", err)
	}

	b, err := xml.MarshalIndent(rs, "", "\t")
	if err != nil {
		l.Fatalf("error serializing xml: %q", err)
	}
	fmt.Println(string(b))
}
