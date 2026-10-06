package parser

import "testing"

func TestParseMTProtoText(t *testing.T) {
	content := `# telegram proxies
tg://proxy?server=1.2.3.4&port=443&secret=eeb87dc15f92c391155eccb9293e2881796f7a6f6e2e7275
tg://socks?server=5.6.7.8&port=1080
host.example:443:aabbccddeeff00112233445566778899
example.com:8080
t.me/proxy?server=9.9.9.9&port=443&secret=ddaabbccddeeff00112233445566778899`

	got := ParseMTProtoText(content)
	if len(got) != 4 {
		t.Fatalf("ожидалось 4 Telegram-прокси, получено %d: %+v", len(got), got)
	}
	if got[0].Type != "mtproto" || got[0].Secret == "" || got[0].Host != "1.2.3.4" || got[0].Port != 443 {
		t.Errorf("tg://proxy разобран неверно: %+v", got[0])
	}
	if got[1].Type != "socks" || got[1].Host != "5.6.7.8" || got[1].Port != 1080 {
		t.Errorf("tg://socks разобран неверно: %+v", got[1])
	}
	if got[2].Type != "mtproto" || got[2].Secret != "aabbccddeeff00112233445566778899" {
		t.Errorf("host:port:secret разобран неверно: %+v", got[2])
	}
	if got[3].Host != "9.9.9.9" {
		t.Errorf("t.me/proxy разобран неверно: %+v", got[3])
	}
}

func TestParseTextSkipsMTProto(t *testing.T) {
	// Обычный парсер не должен брать Telegram-строки.
	got, err := ParseText("example.com:8080\nhost.example:443:aabbccddeeff00112233445566778899\ntg://proxy?server=1.2.3.4&port=443&secret=aabb")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Host != "example.com" {
		t.Fatalf("обычный парсер должен взять только example.com:8080, получено %+v", got)
	}
}
