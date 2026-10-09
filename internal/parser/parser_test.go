package parser

import "testing"

func TestParseText(t *testing.T) {
	content := `# комментарий
// ещё комментарий

192.168.1.1:8080
socks5://10.0.0.1:1080
https://203.0.113.5:3128
socks4://1.2.3.4:1080
некорректная строка
http://host.example:80/path
192.168.1.1:8080`

	got, err := ParseText(content)
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 socks5-прокси (http/https/socks4 пропускаются), получено %d: %+v", len(got), got)
	}

	want := map[string]struct {
		port     int
		protocol string
	}{
		"192.168.1.1": {8080, "socks5"}, // строка без схемы считается socks5
		"10.0.0.1":    {1080, "socks5"},
	}
	for _, p := range got {
		w, ok := want[p.Host]
		if !ok {
			t.Errorf("неожиданный host %q", p.Host)
			continue
		}
		if p.Port != w.port || p.Protocol != w.protocol {
			t.Errorf("%s: получено %s:%d, ожидалось %s:%d", p.Host, p.Protocol, p.Port, w.protocol, w.port)
		}
	}
}

func TestParseTextLowercasesHost(t *testing.T) {
	got, err := ParseText("EXAMPLE.COM:8080")
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}
	if len(got) != 1 || got[0].Host != "example.com" {
		t.Fatalf("ожидался host в нижнем регистре, получено %+v", got)
	}
}

func TestParseJSONArray(t *testing.T) {
	content := []byte(`[
		{"ip":"1.1.1.1","port":8080,"protocols":["http"],"country":"US"},
		{"ip":"2.2.2.2","port":"1080","protocol":"socks5"},
		{"ip":"3.3.3.3","port":3128,"protocols":["socks4"]}
	]`)

	got, err := ParseJSON(content)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ожидалось 1 socks5-прокси (http/socks4 пропускаются), получено %d: %+v", len(got), got)
	}
	if got[0].Protocol != "socks5" || got[0].Port != 1080 || got[0].Host != "2.2.2.2" {
		t.Errorf("запись разобрана неверно: %+v", got[0])
	}
}

func TestParseJSONL(t *testing.T) {
	content := []byte(`{"ip":"1.1.1.1","port":8080,"protocols":["http"]}
{"ip":"2.2.2.2","port":1080,"protocols":["socks5"]}`)

	got, err := ParseJSON(content)
	if err != nil {
		t.Fatalf("ParseJSON (JSONL): %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ожидалось 1 socks5-прокси (http пропускается), получено %d", len(got))
	}
}
