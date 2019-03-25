package fluconf

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name   string
		shared Config
		conf   string
		want   []Config
	}{
		{
			"n1", nil,
			`http uri=/xxx/xxx port=80 
			tcp port=80
			exec command="bash -c 'echo foo'" timeout=2000 interval=10000
			`,
			[]Config{
				{"probe": "http", "uri": "/xxx/xxx", "port": "80"},
				{"probe": "tcp", "port": "80"},
				{"probe": "exec", "command": "bash -c 'echo foo'", "timeout": "2000", "interval": "10000"},
			},
		},
		{
			"n2", nil,
			`timeout=5000 interval=5000 fall=3 rise=2
			http 
				uri=/xxx/xxx
				port=80 
			tcp
				port=80
			exec 
				command="bash -c 'echo foo'" 
				timeout=2000 
				interval=10000
				fall=1
				rise=1`,
			[]Config{
				{"probe": "http", "uri": "/xxx/xxx", "port": "80", "timeout": "5000", "interval": "5000", "fall": "3", "rise": "2"},
				{"probe": "tcp", "port": "80", "timeout": "5000", "interval": "5000", "fall": "3", "rise": "2"},
				{"probe": "exec", "command": "bash -c 'echo foo'", "timeout": "2000", "interval": "10000", "fall": "1", "rise": "1"},
			},
		},
		{
			"n3", Config{"timeout": "5000", "interval": "5000", "fall": "3", "rise": "2"},
			`http 
				uri=/xxx/xxx
				port=80 
			tcp
				port=80
			exec 
				command="bash -c 'echo foo'" 
				timeout=2000 
				interval=10000
				fall=1
				rise=1`,
			[]Config{
				{"probe": "http", "uri": "/xxx/xxx", "port": "80", "timeout": "5000", "interval": "5000", "fall": "3", "rise": "2"},
				{"probe": "tcp", "port": "80", "timeout": "5000", "interval": "5000", "fall": "3", "rise": "2"},
				{"probe": "exec", "command": "bash -c 'echo foo'", "timeout": "2000", "interval": "10000", "fall": "1", "rise": "1"},
			},
		},
		{
			"n4", nil,
			``,
			[]Config{},
		},
		{
			"n5", nil,
			`abc=123 cde=test`,
			[]Config{},
		},
		{
			"n6", nil,
			`abc=123 test =invalid test=def`,
			[]Config{{"probe": "test", "abc": "123", "test": "def"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.conf, "probe", tt.shared); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_Get(t *testing.T) {
	c := Config{"int": "12", "eint": "n/a", "str": "str"}
	t.Run("Gets", func(t *testing.T) {
		if got, want := c.GetInt("int", 0), 12; got != want {
			t.Errorf("Config.GetInt(int) = %v, want %v", got, want)
		}
		if got, want := c.GetInt("eint", 99), 99; got != want {
			t.Errorf("Config.GetInt(eint) = %v, want %v", got, want)
		}
		if got, want := c.GetString("str", "str"), "str"; got != want {
			t.Errorf("Config.GetString(str) = %v, want %v", got, want)
		}
		if got, want := c.GetString("nostr", "nostr"), "nostr"; got != want {
			t.Errorf("Config.GetString(nostr) = %v, want %v", got, want)
		}
	})
}
