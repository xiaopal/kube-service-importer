package fluconf

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

func tokenize(conf string) []string {
	insideQuote, quoteChar := false, rune(0)
	fieldsFunc := func(c rune) bool {
		if insideQuote {
			if c == quoteChar {
				insideQuote, quoteChar = false, rune(0)
			}
			return false
		}
		if unicode.In(c, unicode.Quotation_Mark) {
			insideQuote, quoteChar = true, c
			return false
		}
		return unicode.IsSpace(c)
	}
	return strings.FieldsFunc(conf, fieldsFunc)
}

func parseToken(token string) (key string, val string, ok bool) {
	if ss := strings.SplitN(token, "=", 2); len(ss) == 1 {
		val = ss[0]
	} else if key, val = ss[0], ss[1]; key == "" {
		return "", "", false
	}
	if len(val) > 0 && unicode.In(rune(val[0]), unicode.Quotation_Mark) {
		if vv, err := strconv.Unquote(val); err == nil {
			val = vv
		}
	}
	return key, val, true
}

// Parse func
func Parse(conf string, entryKey string, shared Config) []Config {
	if shared == nil {
		shared = Config{}
	}
	entry, entries := shared, []Config{}
	for _, token := range tokenize(conf) {
		if key, val, ok := parseToken(token); ok {
			if key == "" {
				entry = shared.CopyWith(entryKey, val)
				entries = append(entries, entry)
			} else {
				entry[key] = val
			}
		}
	}
	return entries
}

// Config type
type Config map[string]string

// GetString func
func (c Config) GetString(name, defaultVal string) string {
	if sval, ok := c[name]; ok {
		return sval
	}
	return defaultVal
}

// GetInt func
func (c Config) GetInt(name string, defaultVal int) int {
	if sval, ok := c[name]; ok {
		if ival, err := strconv.Atoi(sval); err == nil {
			return ival
		}
	}
	return defaultVal
}

// GetBool func
func (c Config) GetBool(name string, defaultVal bool) bool {
	if sval, ok := c[name]; ok {
		switch strings.ToLower(sval) {
		case "true", "yes", "1", "t", "y":
			return true
		case "false", "no", "0", "f", "n", "-1":
			return false
		}
	}
	return defaultVal
}

// GetDuration func
func (c Config) GetDuration(name string, defaultVal time.Duration) time.Duration {
	if sval, ok := c[name]; ok {
		if dval, err := time.ParseDuration(sval); err == nil {
			return dval
		}
	}
	return defaultVal
}

// CopyWithAll func
func (c Config) CopyWithAll(overwrites Config) Config {
	ret := c.Copy()
	for k, v := range overwrites {
		ret[k] = v
	}
	return ret
}

// CopyWith func
func (c Config) CopyWith(name string, val string) Config {
	return c.CopyWithAll(Config{name: val})
}

// Copy func
func (c Config) Copy() Config {
	ret := Config{}
	for k, v := range c {
		ret[k] = v
	}
	return ret
}
