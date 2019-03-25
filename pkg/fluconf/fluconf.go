/*Package fluconf implements simple and fluent configuration reader
  eg.
	conf := `timeout=5000 interval=5000 fall=3 rise=2
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
				rise=1`
	fluconf.Parse(conf, "probe") ->
			[]map[string]string{
				{"probe": "http", "uri": "/xxx/xxx", "port": "80", "timeout": "5000", "interval": "5000", "fall": "3", "rise": "2"},
				{"probe": "tcp", "port": "80", "timeout": "5000", "interval": "5000", "fall": "3", "rise": "2"},
				{"probe": "exec", "command": "bash -c 'echo foo'", "timeout": "2000", "interval": "10000", "fall": "1", "rise": "1"}}
*/package fluconf
