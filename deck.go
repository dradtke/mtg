package mtg

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	decLineRe = regexp.MustCompile(`^(\d+) (.+)$`)

	ErrDeckTooSmall = errors.New("deck is too small")
)

// A deck represents your Magic deck. The Main field maps from card name
// to how many of them are in the deck, and Sideboard does the same for
// cards in your sideboard.
type Deck struct {
	Main      map[Card]int
	Sideboard map[Card]int
}

// NewDeck creates a new deck from the provided reader, which should provide
// deck information in .dec format.
func NewDeck(r io.Reader) (Deck, error) {
	main, sideboard := make(map[string]int), make(map[string]int)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var (
			line        = strings.TrimSpace(scanner.Text())
			isSideboard bool
		)
		if line == "" {
			continue
		}
		if len(line) > 3 && line[:3] == "SB:" {
			isSideboard = true
			line = strings.TrimSpace(line[3:])
		}

		count, cardName, err := parseCardLine(line)
		if err != nil {
			return Deck{}, err
		}

		if isSideboard {
			sideboard[cardName] += count
		} else {
			main[cardName] += count
		}

	}

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		deck = Deck{
			Main:      make(map[Card]int),
			Sideboard: make(map[Card]int),
		}
	)

	wg.Add(len(main) + len(sideboard))
	for cardName, count := range main {
		go func(cardName string, count int) {
			defer wg.Done()
			card, err := GetCardForName(cardName)
			if err != nil {
				fmt.Println("failed to find card " + cardName + ": " + err.Error())
				return
			}

			mu.Lock()
			deck.Main[card] += count
			mu.Unlock()
		}(cardName, count)
	}

	for cardName, count := range sideboard {
		go func(cardName string, count int) {
			defer wg.Done()
			card, err := GetCardForName(cardName)
			if err != nil {
				fmt.Println("failed to find card " + cardName + ": " + err.Error())
				return
			}

			mu.Lock()
			deck.Sideboard[card] += count
			mu.Unlock()
		}(cardName, count)
	}

	wg.Wait()
	return deck, scanner.Err()
}

func (d Deck) Colors() []string {
	m := make(map[string]struct{})
	for card := range d.Main {
		for _, color := range card.Colors() {
			m[color] = struct{}{}
		}
	}

	var colors []string
	for _, color := range allColors {
		if _, ok := m[color]; ok {
			colors = append(colors, color)
		}
	}
	return colors
}

func (d Deck) Size() (size int) {
	for _, n := range d.Main {
		size += n
	}
	return
}

func (d Deck) Lands() (map[Card]int, int) {
	var (
		lands = make(map[Card]int)
		total int
	)
	for card, count := range d.Main {
		if card.Type == "Land" || card.Type == "Basic Land" || strings.HasPrefix(card.Type, "Land ") || strings.HasPrefix(card.Type, "Basic Land ") {
			lands[card] = count
			total += count
		}
	}
	return lands, total
}

type ErrCardLimitExceeded struct {
	Card string
}

func (e ErrCardLimitExceeded) Error() string {
	return "too many copies of: " + e.Card
}

type Format int

const (
	_ Format = iota
	Constructed
	Limited
)

func (d Deck) Validate(format Format) error {
	switch format {
	case Constructed:
		if d.Size() < 60 {
			return ErrDeckTooSmall
		}
		for card, count := range d.Main {
			// TODO: add check for basic land
			if count > 4 {
				return ErrCardLimitExceeded{card.Name}
			}
		}
		return nil

	case Limited:
		if d.Size() < 40 {
			return ErrDeckTooSmall
		}
		return nil

	default:
		return errors.New("unknown format")
	}
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
