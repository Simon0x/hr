package statement

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Compact JSON, source key order preserved. Matches JSON.stringify(JSON.parse(raw)) except for integer-like string keys, which JS reorders and this does not; no hr schema uses them as object keys today.
func Canonical(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var buf bytes.Buffer
	if err := canonValue(dec, &buf); err != nil {
		return nil, err
	}

	if dec.More() {
		return nil, fmt.Errorf("trailing data after JSON value")
	}
	var trailing any
	if err := dec.Decode(&trailing); err == nil {
		return nil, fmt.Errorf("trailing data after JSON value")
	}

	return buf.Bytes(), nil
}

func Digest(raw []byte) (string, error) {
	c, err := Canonical(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(c)
	return hex.EncodeToString(sum[:]), nil
}

func canonValue(dec *json.Decoder, buf *bytes.Buffer) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	return canonToken(tok, dec, buf)
}

func canonToken(tok json.Token, dec *json.Decoder, buf *bytes.Buffer) error {
	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			return canonObject(dec, buf)
		case '[':
			return canonArray(dec, buf)
		default:
			return fmt.Errorf("unexpected delimiter %q", v)
		}
	case string:
		s, err := marshalString(v)
		if err != nil {
			return err
		}
		buf.Write(s)
	case json.Number:
		buf.WriteString(v.String())
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case nil:
		buf.WriteString("null")
	default:
		return fmt.Errorf("unexpected token type %T", tok)
	}
	return nil
}

func canonObject(dec *json.Decoder, buf *bytes.Buffer) error {
	buf.WriteByte('{')
	first := true
	for dec.More() {
		if !first {
			buf.WriteByte(',')
		}
		first = false

		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("expected object key, got %v", keyTok)
		}
		kb, err := marshalString(key)
		if err != nil {
			return err
		}
		buf.Write(kb)
		buf.WriteByte(':')

		if err := canonValue(dec, buf); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	buf.WriteByte('}')
	return nil
}

func canonArray(dec *json.Decoder, buf *bytes.Buffer) error {
	buf.WriteByte('[')
	first := true
	for dec.More() {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		if err := canonValue(dec, buf); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	buf.WriteByte(']')
	return nil
}

func marshalString(s string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
