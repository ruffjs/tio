package redissplit

import (
	"encoding/hex"
	"errors"
	"fmt"
	"unicode"
)

// Split line to args in array for Redis
// Ref： https://github.com/qishibo/splitargs

func SplitArgs(line string) ([]string, error) {
	var ret []string
	if line == "" {
		return []string{}, nil
	}

	len := len(line)
	pos := 0
	for pos < len {
		// Skip blanks
		for pos < len && isSpace(line[pos]) {
			pos += 1
		}

		if pos == len {
			break
		}

		inq := false  // If we are in "quotes"
		insq := false // If we are in "single quotes"
		done := false
		var current []byte

		for !done {
			c := line[pos]
			if inq {
				if c == '\\' && (pos+1) < len {
					pos += 1

					switch line[pos] {
					case 'n':
						c = '\n'
					case 'r':
						c = '\r'
					case 't':
						c = '\t'
					case 'b':
						c = '\b'
					case 'a':
						c = '\a'
					case 'x':
						hexStr := line[pos+1 : pos+3]
						decoded, err := hex.DecodeString(hexStr)
						if err == nil {
							c = decoded[0]
							pos += 2
							break
						}
						// Hex decoding failed, remove "\" and include the original character
						c = line[pos]
					default:
						c = line[pos]
					}
					current = append(current, c)
				} else if c == '"' {
					// Closing quote must be followed by a space or nothing at all.
					if pos+1 < len && !isSpace(line[pos+1]) {
						return nil, fmt.Errorf("expect '\"' followed by a space or nothing, got '%c'", line[pos+1])
					}
					done = true
				} else if pos == len-1 {
					return nil, fmt.Errorf("unterminated quotes")
				} else {
					current = append(current, c)
				}
			} else if insq {
				if c == '\\' && line[pos+1] == '\'' {
					pos += 1
					current = append(current, '\'')
				} else if c == '\'' {
					// Closing quote must be followed by a space or nothing at all.
					if pos+1 < len && !isSpace(line[pos+1]) {
						panic(errors.New(fmt.Sprintf("Expect \"'\" followed by a space or nothing, got \"%c\".", line[pos+1])))
					}
					done = true
				} else if pos == len-1 {
					return nil, fmt.Errorf("unterminated quotes")
				} else {
					current = append(current, c)
				}
			} else {
				if pos == len-1 {
					done = true
				}

				switch c {
				case ' ', '\n', '\r', '\t':
					done = true
				case '"':
					inq = true
				case '\'':
					insq = true
				default:
					current = append(current, c)
				}
			}
			if pos < len {
				pos += 1
			}
		}
		ret = append(ret, string(current))
	}

	return ret, nil
}

func isSpace(ch byte) bool {
	return unicode.IsSpace(rune(ch))
}
