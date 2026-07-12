package rule

func leb128(v uint32) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

func section(id byte, data []byte) []byte {
	out := make([]byte, 0, 1+len(data)+5)
	out = append(out, id)
	out = append(out, leb128(uint32(len(data)))...)
	out = append(out, data...)
	return out
}

func nameStr(n string) []byte {
	return append([]byte{byte(len(n))}, n...)
}

func funcBody(code ...byte) []byte {
	payload := make([]byte, 0, 1+len(code)+1)
	payload = append(payload, 0)
	payload = append(payload, code...)
	payload = append(payload, 0x0b)
	out := leb128(uint32(len(payload)))
	return append(out, payload...)
}

func BuildSimpleTestWASM(name, desc string) []byte {
	nameNUL := append([]byte(name), 0)
	descNUL := append([]byte(desc), 0)
	data := append(nameNUL, descNUL...)

	nameOff := 0
	descOff := len(nameNUL)

	var buf []byte
	buf = append(buf, 0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00)

	// Type section:
	// type 0: () -> i32       (rule_name, rule_description, rule_init)
	// type 1: (i32, i32) -> () (host_log, host_alert)
	// type 2: (i32, i32) -> i32 (rule_evaluate)
	buf = append(buf, section(1, func() []byte {
		var d []byte
		d = append(d, 3)
		d = append(d, 0x60, 0x00, 0x01, 0x7f)
		d = append(d, 0x60, 0x02, 0x7f, 0x7f, 0x00)
		d = append(d, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f)
		return d
	}())...)

	// Import section: 2 functions from "env"
	buf = append(buf, section(2, func() []byte {
		var d []byte
		d = append(d, 2)
		d = append(d, nameStr("env")...)
		d = append(d, nameStr("host_log")...)
		d = append(d, 0x00, 0x01)
		d = append(d, nameStr("env")...)
		d = append(d, nameStr("host_alert")...)
		d = append(d, 0x00, 0x01)
		return d
	}())...)

	// Function section: 4 functions (indices 2..5)
	// func 2: rule_name        -> type 0
	// func 3: rule_description -> type 0
	// func 4: rule_init        -> type 0
	// func 5: rule_evaluate    -> type 2
	buf = append(buf, section(3, func() []byte {
		var d []byte
		d = append(d, 4)
		d = append(d, 0x00)
		d = append(d, 0x00)
		d = append(d, 0x00)
		d = append(d, 0x02)
		return d
	}())...)

	// Memory section: 1 page, no max
	buf = append(buf, section(5, []byte{0x01, 0x00, 0x01})...)

	// Export section
	buf = append(buf, section(7, func() []byte {
		var d []byte
		d = append(d, 5)
		d = append(d, nameStr("memory")...)
		d = append(d, 0x02, 0x00)
		d = append(d, nameStr("rule_name")...)
		d = append(d, 0x00, 0x02)
		d = append(d, nameStr("rule_description")...)
		d = append(d, 0x00, 0x03)
		d = append(d, nameStr("rule_init")...)
		d = append(d, 0x00, 0x04)
		d = append(d, nameStr("rule_evaluate")...)
		d = append(d, 0x00, 0x05)
		return d
	}())...)

	// Code section: 4 function bodies
	buf = append(buf, section(10, func() []byte {
		var d []byte
		d = append(d, 4)
		d = append(d, funcBody(0x41, byte(nameOff))...)
		d = append(d, funcBody(0x41, byte(descOff))...)
		d = append(d, funcBody(0x41, 0x00)...)
		d = append(d, funcBody(0x41, 0x00)...)
		return d
	}())...)

	// Data section: init memory with name + desc strings
	buf = append(buf, section(11, func() []byte {
		var d []byte
		d = append(d, 0x01)
		d = append(d, 0x00)
		d = append(d, 0x41, 0x00, 0x0b)
		d = append(d, leb128(uint32(len(data)))...)
		d = append(d, data...)
		return d
	}())...)

	return buf
}

func BuildEvalTestWASM(name, desc string, alwaysAlert bool) []byte {
	nameNUL := append([]byte(name), 0)
	descNUL := append([]byte(desc), 0)
	data := append(nameNUL, descNUL...)

	nameOff := 0
	descOff := len(nameNUL)

	var alertBody []byte
	if alwaysAlert {
		alertBody = funcBody(0x41, 0x01)
	} else {
		alertBody = funcBody(0x41, 0x00)
	}

	var buf []byte
	buf = append(buf, 0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00)

	// Type section
	buf = append(buf, section(1, func() []byte {
		var d []byte
		d = append(d, 3)
		d = append(d, 0x60, 0x00, 0x01, 0x7f)
		d = append(d, 0x60, 0x02, 0x7f, 0x7f, 0x00)
		d = append(d, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f)
		return d
	}())...)

	// Import section: 2 functions from "env"
	buf = append(buf, section(2, func() []byte {
		var d []byte
		d = append(d, 2)
		d = append(d, nameStr("env")...)
		d = append(d, nameStr("host_log")...)
		d = append(d, 0x00, 0x01)
		d = append(d, nameStr("env")...)
		d = append(d, nameStr("host_alert")...)
		d = append(d, 0x00, 0x01)
		return d
	}())...)

	// Function section: 4 functions (indices 3..5)
	buf = append(buf, section(3, func() []byte {
		var d []byte
		d = append(d, 4)
		d = append(d, 0x00)
		d = append(d, 0x00)
		d = append(d, 0x00)
		d = append(d, 0x02)
		return d
	}())...)

	// Memory section
	buf = append(buf, section(5, []byte{0x01, 0x00, 0x01})...)

	// Export section
	buf = append(buf, section(7, func() []byte {
		var d []byte
		d = append(d, 5)
		d = append(d, nameStr("memory")...)
		d = append(d, 0x02, 0x00)
		d = append(d, nameStr("rule_name")...)
		d = append(d, 0x00, 0x02)
		d = append(d, nameStr("rule_description")...)
		d = append(d, 0x00, 0x03)
		d = append(d, nameStr("rule_init")...)
		d = append(d, 0x00, 0x04)
		d = append(d, nameStr("rule_evaluate")...)
		d = append(d, 0x00, 0x05)
		return d
	}())...)

	// Code section
	buf = append(buf, section(10, func() []byte {
		var d []byte
		d = append(d, 4)
		d = append(d, funcBody(0x41, byte(nameOff))...)
		d = append(d, funcBody(0x41, byte(descOff))...)
		d = append(d, funcBody(0x41, 0x00)...)
		d = append(d, alertBody...)
		return d
	}())...)

	// Data section
	buf = append(buf, section(11, func() []byte {
		var d []byte
		d = append(d, 0x01)
		d = append(d, 0x00)
		d = append(d, 0x41, 0x00, 0x0b)
		d = append(d, leb128(uint32(len(data)))...)
		d = append(d, data...)
		return d
	}())...)

	return buf
}
