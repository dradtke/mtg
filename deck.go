package mtg

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var decLineRe = regexp.MustCompile(`^(\d+) (.+)$`)

// A deck represents your Magic deck. The Main field maps from card name
// to how many of them are in the deck, and Sideboard does the same for
// cards in your sideboard.
type Deck struct {
	Main      map[string]int
	Sideboard map[string]int
}

// NewDeck creates a new deck from the provided reader, which should provide
// deck information in .dec format.
func NewDeck(r io.Reader) (Deck, error) {
	deck := Deck{Main: make(map[string]int), Sideboard: make(map[string]int)}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var (
			line      = strings.TrimSpace(scanner.Text())
			sideboard bool
		)
		if line == "" {
			continue
		}
		if len(line) > 3 && line[:3] == "SB:" {
			sideboard = true
			line = strings.TrimSpace(line[3:])
		}

		n, c, err := parseCardLine(line)
		if err != nil {
			return deck, err
		}

		if sideboard {
			deck.Sideboard[c] = n
		} else {
			deck.Main[c] = n
		}
	}

	return deck, scanner.Err()
}

func (d Deck) String() string {
	var buf bytes.Buffer
	for c, n := range d.Main {
		buf.WriteString(fmt.Sprintf("%d %s\n", n, c))
	}
	if len(d.Sideboard) > 0 {
		buf.WriteString("\nSideboard:\n")
		for c, n := range d.Sideboard {
			buf.WriteString(fmt.Sprintf("%d %s\n", n, c))
		}
	}
	return buf.String()
}

func parseCardLine(line string) (int, string, error) {
	matches := decLineRe.FindStringSubmatch(line)
	if matches == nil {
		return 0, "", fmt.Errorf("line '%s' is not a valid card definition", line)
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, "", err
	}

	return n, matches[2], nil
}
